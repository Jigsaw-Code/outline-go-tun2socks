// Copyright 2022 The Outline Authors
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

package dns

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
)

// TCPDialer is a core TCP handler that captures DNS connections
type TCPDialer interface {
	core.TCPDialer
	SetDNS(Transport)
}

type dnsOnlyDialer struct {
	dns Atomic
}

func NewTCPDialer() TCPDialer {
	return &dnsOnlyDialer{}
}

func (d *dnsOnlyDialer) DialTCP(target *net.TCPAddr) (core.DoubleStream, error) {
	log.Printf("Handling DNS-TCP for %s", target.String())
	if target.Port != 53 {
		return nil, errors.New("only plain DNS over TCP is supported")
	}
	dns := d.dns.Load()
	return Accept(dns), nil
}

func (d *dnsOnlyDialer) SetDNS(dns Transport) {
	d.dns.Store(dns)
}

type UDPListener interface {
	core.UDPListener
	SetDNS(Transport)
}

type dnsOnlyListener struct {
	dns Atomic
}

func NewUDPListener() UDPListener {
	return &dnsOnlyListener{}
}

func (l *dnsOnlyListener) ListenUDP(reverse core.ReverseUDPConn) (net.PacketConn, error) {
	return makeDnsOnlyConn(l, reverse), nil
}

func (l *dnsOnlyListener) SetDNS(dns Transport) {
	l.dns.Store(dns)
}

type dnsOnlyConn struct {
	listener      *dnsOnlyListener
	reverse       core.ReverseUDPConn
	fakeReader    net.Conn
	mu            sync.RWMutex
	pending       int
	writeDeadline time.Time
	closed        bool
}

func makeDnsOnlyConn(listener *dnsOnlyListener, reverse core.ReverseUDPConn) net.PacketConn {
	p1, _ := net.Pipe()
	return &dnsOnlyConn{
		listener:   listener,
		reverse:    reverse,
		fakeReader: p1,
	}
}

func (c *dnsOnlyConn) WriteTo(pkt []byte, addr net.Addr) (int, error) {
	log.Printf("Handling DNS-UDP packet for %s", addr.String())
	udpAddr, err := core.UDPAddr(addr)
	if err != nil {
		return 0, err
	}
	if udpAddr.Port != 53 {
		return 0, errors.New("only plain DNS over UDP is supported")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.EOF
	}
	running := c.writeDeadline.Equal(time.Time{}) || time.Now().Before(c.writeDeadline)
	if !running {
		return 0, errors.New("timeout")
	}
	c.pending++
	go func() {
		dns := c.listener.dns.Load()
		resp, err := dns.Query(pkt)
		if err != nil {
			log.Printf("query failed: %v", err)
			resp = TryServfail(pkt)
		}
		if len(resp) > 0 {
			c.reverse.WriteFrom(resp, udpAddr)
		}

		c.mu.Lock()
		c.pending--
		if c.pending == 0 {
			c.Close()
		}
		c.mu.Unlock()
	}()
	return len(pkt), nil
}

func (c *dnsOnlyConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	_, err := c.fakeReader.Read(buf)
	return 0, nil, err
}

func (c *dnsOnlyConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeDeadline = t
	c.mu.Unlock()
	return nil
}

func (c *dnsOnlyConn) SetReadDeadline(t time.Time) error {
	return c.fakeReader.SetReadDeadline(t)
}

func (c *dnsOnlyConn) SetDeadline(t time.Time) error {
	c.SetWriteDeadline(t)
	c.SetReadDeadline(t)
	return nil
}

func (c *dnsOnlyConn) Close() error {
	c.fakeReader.Close()
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *dnsOnlyConn) LocalAddr() net.Addr {
	return nil
}
