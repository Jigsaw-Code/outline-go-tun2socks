// Copyright 2019 The Outline Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Derived from go-tun2socks's "direct" handler under the Apache 2.0 license.

package intra

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"

	"github.com/Jigsaw-Code/outline-go-tun2socks/intra/doh"
)

// UDPSocketSummary describes a non-DNS UDP association, reported when it is discarded.
type UDPSocketSummary struct {
	UploadBytes   int64 // Amount uploaded (bytes)
	DownloadBytes int64 // Amount downloaded (bytes)
	Duration      int32 // How long the socket was open (seconds)
}

// UDPListener is notified when a non-DNS UDP association is discarded.
type UDPListener interface {
	OnUDPSocketClosed(*UDPSocketSummary)
}

type tracker struct {
	conn     *net.UDPConn
	start    time.Time
	upload   int64 // Non-DNS upload bytes
	download int64 // Non-DNS download bytes
}

func makeTracker(conn *net.UDPConn) *tracker {
	return &tracker{conn, time.Now(), 0, 0}
}

// UDPHandler adds DOH support to the base UDPConnHandler interface.
type UDPHandler interface {
	core.UDPConnHandler
	SetDNS(dns doh.Transport)
}

type udpHandler struct {
	UDPHandler
	sync.RWMutex

	timeout  time.Duration
	udpConns map[core.UDPConn]*tracker
	fakedns  net.UDPAddr
	dns      doh.Transport
	config   *net.ListenConfig
	listener UDPListener
}

// NewUDPHandler makes a UDP handler with Intra-style DNS redirection:
// All packets are routed directly to their destination, except packets whose
// destination is `fakedns`.  Those packets are redirected to DOH.
// `timeout` controls the effective NAT mapping lifetime.
// `config` is used to bind new external UDP ports.
// `listener` receives a summary about each UDP binding when it expires.
func NewUDPHandler(fakedns net.UDPAddr, timeout time.Duration, config *net.ListenConfig, listener UDPListener) UDPHandler {
	return &udpHandler{
		timeout:  timeout,
		udpConns: make(map[core.UDPConn]*tracker, 8),
		fakedns:  fakedns,
		config:   config,
		listener: listener,
	}
}

func (h *udpHandler) fetchUDPInput(conn core.UDPConn, t *tracker) {
	buf := core.NewBytes(core.BufSize)

	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()

	for {
		t.conn.SetDeadline(time.Now().Add(h.timeout))
		n, addr, err := t.conn.ReadFrom(buf)
		if err != nil {
			return
		}

		udpaddr := addr.(*net.UDPAddr)
		t.download += int64(n)
		_, err = conn.WriteFrom(buf[:n], udpaddr)
		if err != nil {
			log.Warnf("failed to write UDP data to TUN")
			return
		}
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	bindAddr := &net.UDPAddr{IP: nil, Port: 0}
	pc, err := h.config.ListenPacket(context.TODO(), bindAddr.Network(), bindAddr.String())
	if err != nil {
		log.Errorf("failed to bind udp address")
		return err
	}
	t := makeTracker(pc.(*net.UDPConn))
	h.Lock()
	h.udpConns[conn] = t
	h.Unlock()
	go h.fetchUDPInput(conn, t)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *udpHandler) doDoh(dns doh.Transport, t *tracker, conn core.UDPConn, data []byte) {
	resp, err := dns.Query(data)
	if err == nil {
		_, err = conn.WriteFrom(resp, &h.fakedns)
	}
	if err != nil {
		log.Warnf("DoH query failed: %v", err)
	}
	// Note: Reading t.upload and t.download on this thread, while they are written on
	// other threads, is theoretically a race condition.  In practice, this race is
	// impossible on 64-bit platforms, likely impossible on 32-bit platforms, and
	// low-impact if it occurs (a mixed-use socket might be closed early).
	if t.upload == 0 && t.download == 0 {
		// conn was only used for this DNS query, so it's unlikely to be used again.
		h.Close(conn)
	}
}

func (h *udpHandler) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	h.RLock()
	dns := h.dns
	t, ok1 := h.udpConns[conn]
	h.RUnlock()

	if !ok1 {
		return fmt.Errorf("connection %v->%v does not exists", conn.LocalAddr(), addr)
	}

	// Update deadline.
	t.conn.SetDeadline(time.Now().Add(h.timeout))

	if addr.IP.Equal(h.fakedns.IP) && addr.Port == h.fakedns.Port {
		dataCopy := append([]byte{}, data...)
		go h.doDoh(dns, t, conn, dataCopy)
		return nil
	}
	t.upload += int64(len(data))
	_, err := t.conn.WriteTo(data, addr)
	if err != nil {
		log.Warnf("failed to forward UDP payload")
		return errors.New("failed to write UDP data")
	}
	return nil
}

func (h *udpHandler) Close(conn core.UDPConn) {
	conn.Close()

	h.Lock()
	defer h.Unlock()

	if t, ok := h.udpConns[conn]; ok {
		t.conn.Close()
		// TODO: Cancel any outstanding DoH queries.
		duration := int32(time.Since(t.start).Seconds())
		h.listener.OnUDPSocketClosed(&UDPSocketSummary{t.upload, t.download, duration})
		delete(h.udpConns, conn)
	}
}

func (h *udpHandler) SetDNS(dns doh.Transport) {
	h.Lock()
	h.dns = dns
	h.Unlock()
}
