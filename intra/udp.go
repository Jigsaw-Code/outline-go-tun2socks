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
	"net"
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/common/log"

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
	start    time.Time
	upload   int64 // Non-DNS upload bytes
	download int64 // Non-DNS download bytes
}

type packetConnWrapper struct {
	net.PacketConn
	tracker
	reverse  core.ReverseUDPConn
	listener *udpListener

	mu                          sync.Mutex // TODO: Optimize with atomics
	pendingQueries              int
	readDeadline, writeDeadline time.Time
	closed                      bool
}

func (c *packetConnWrapper) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(buf)
	c.tracker.download += int64(n)
	return n, addr, err
}

func (c *packetConnWrapper) WriteTo(p []byte, addr net.Addr) (int, error) {
	udpAddr := addr.(*net.UDPAddr)
	if udpAddr.IP.Equal(c.listener.fakedns.IP) && udpAddr.Port == c.listener.fakedns.Port {
		go c.doDoh(p)
		return len(p), nil
	}
	n, err := c.WriteTo(p, addr)
	c.tracker.upload += int64(n)
	return n, err
}

func (c *packetConnWrapper) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = deadline
	if c.pendingQueries == 0 {
		return c.PacketConn.SetReadDeadline(deadline)
	}
	return nil
}

func (c *packetConnWrapper) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = deadline
	if c.pendingQueries == 0 {
		return c.PacketConn.SetWriteDeadline(deadline)
	}
	return nil
}

func (c *packetConnWrapper) SetDeadline(deadline time.Time) error {
	// TODO: Consider atomicity.
	c.SetReadDeadline(deadline)
	c.SetWriteDeadline(deadline)
	return nil // FIXME
}

func (c *packetConnWrapper) doDoh(pkt []byte) {
	c.mu.Lock()
	c.pendingQueries++
	// Disable timeouts while a DoH query is pending
	c.PacketConn.SetDeadline(time.Time{})
	c.mu.Unlock()

	dataCopy := append([]byte{}, pkt...)
	go func() {
		resp, err := c.listener.dns.Query(dataCopy)
		if resp != nil {
			err = c.reverse.WriteFrom(resp, &c.listener.fakedns)
		}
		if err != nil {
			log.Warnf("DoH query failed: %v", err)
		}

		c.mu.Lock()
		c.pendingQueries--
		if c.pendingQueries == 0 {
			if c.tracker.upload == 0 && c.tracker.download == 0 {
				// conn was only used for DNS, so it's unlikely to be used again.
				c.PacketConn.Close()
				c.closed = true
			} else {
				// Restore timeouts
				c.PacketConn.SetReadDeadline(c.readDeadline)
				c.PacketConn.SetWriteDeadline(c.writeDeadline)
			}
		}
		c.mu.Unlock()
	}()
}

func (c *packetConnWrapper) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	err := c.PacketConn.Close()
	c.closed = true
	// TODO: Cancel any outstanding DoH queries.
	t := c.tracker
	duration := int32(time.Since(t.start).Seconds())
	c.listener.listener.OnUDPSocketClosed(&UDPSocketSummary{t.upload, t.download, duration})
	return err
}

// UDPConnListener adds DOH support to the core.UDPListener interface.
type UDPConnListener interface {
	core.UDPListener
	SetDNS(dns doh.Transport)
}

type udpListener struct {
	sync.RWMutex

	fakedns  net.UDPAddr
	dns      doh.Transport
	config   *net.ListenConfig
	listener UDPListener
}

// NewUDPConnListener makes a UDP listener with Intra-style DNS redirection:
// All packets are routed directly to their destination, except packets whose
// destination is `fakedns`.  Those packets are redirected to DOH.
// `timeout` controls the effective NAT mapping lifetime.
// `config` is used to bind new external UDP ports.
// `listener` receives a summary about each UDP binding when it expires.
func NewUDPConnListener(fakedns net.UDPAddr, config *net.ListenConfig, listener UDPListener) UDPConnListener {
	return &udpListener{
		fakedns:  fakedns,
		config:   config,
		listener: listener,
	}
}

func (h *udpListener) ListenUDP(reverse core.ReverseUDPConn) (net.PacketConn, error) {
	bindAddr := &net.UDPAddr{IP: nil, Port: 0}
	pc, err := h.config.ListenPacket(context.TODO(), bindAddr.Network(), bindAddr.String())
	if err != nil {
		log.Errorf("failed to bind udp address")
		return nil, err
	}

	wrapped := &packetConnWrapper{
		PacketConn: pc,
		tracker:    tracker{start: time.Now()},
		reverse:    reverse,
		listener:   h,
	}
	log.Infof("new proxy connection")
	return wrapped, nil
}

func (h *udpListener) SetDNS(dns doh.Transport) {
	h.Lock()
	h.dns = dns
	h.Unlock()
}
