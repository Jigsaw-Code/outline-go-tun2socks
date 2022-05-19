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
	"net"
	"sync/atomic"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"github.com/Jigsaw-Code/outline-go-tun2socks/dns"
	"github.com/Jigsaw-Code/outline-go-tun2socks/split"
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

type measuringListener struct {
	engine   core.UDPListener
	listener UDPListener
}

func (l measuringListener) ListenUDP(reverse core.ReverseUDPConn) (net.PacketConn, error) {
	conn, err := l.engine.ListenUDP(reverse)
	if err != nil {
		return nil, err
	}
	return &measuringPacketConn{
		PacketConn: conn,
		listener:   l.listener,
		start:      time.Now(),
	}, nil
}

type measuringPacketConn struct {
	net.PacketConn
	listener UDPListener
	start    time.Time
	upload   int64
	download int64
	closed   int32
}

func (c *measuringPacketConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(buf)
	atomic.AddInt64(&c.download, int64(n))
	return n, addr, err
}

func (c *measuringPacketConn) WriteTo(pkt []byte, dst net.Addr) (int, error) {
	n, err := c.PacketConn.WriteTo(pkt, dst)
	atomic.AddInt64(&c.upload, int64(n))
	return n, err
}

func (c *measuringPacketConn) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}
	c.listener.OnUDPSocketClosed(&UDPSocketSummary{
		UploadBytes:   atomic.LoadInt64(&c.upload),
		DownloadBytes: atomic.LoadInt64(&c.download),
		Duration:      int32(time.Since(c.start).Seconds()),
	})
	return c.PacketConn.Close()
}

type udpCombinedListener struct {
	core.UDPListener
	dns dns.UDPListener
}

func (l udpCombinedListener) SetDNS(dns dns.Transport) {
	l.dns.SetDNS(dns)
}

// NewUDPConnListener makes a UDP listener with Intra-style DNS redirection:
// All packets are routed directly to their destination, except packets whose
// destination is `fakedns`.  Those packets are redirected to DOH.
// `timeout` controls the effective NAT mapping lifetime.
// `config` is used to bind new external UDP ports.
// `listener` receives a summary about each UDP binding when it expires.
func NewUDPConnListener(fakedns net.UDPAddr, config *net.ListenConfig, listener UDPListener) dns.UDPListener {
	dnsListener := dns.NewUDPListener()
	return udpCombinedListener{
		UDPListener: split.MakeUDPListener(
			split.MakeOneIPRule(fakedns.IP),
			dnsListener,
			&measuringListener{
				engine:   core.DefaultUDPListener{ListenConfig: config},
				listener: listener,
			}),
		dns: dnsListener,
	}
}
