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

package split

import (
	"log"
	"net"
	"sync/atomic"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
)

// IDEA:
// IP-based splitting policy
// TCP: Meta-dialer that chooses inner dialer based on destination.
// UDP: Listener that returns a meta-DoubleStream.  Initially, this
// DoubleStream contains zero DoubleStreams, and new streams are
// instantiated lazily as packets arrive for each side of the policy.

// DNS-based splitting can be implemented by resolving each name through
// an inner or outer resolution path (per policy), and using the results
// to adjust the IP policy.  Results have to be cached (in memory!) until
// they expire to account for possible caching by the system.  This raises
// concerns about crash rates.  Also, after a crash+restart, the mappings
// will be lost, leading to connection failures.

// A reasonable middle ground may be to keep resolution and routing
// independent.  Then a list of hostnames to proxy, and IP ranges _not_ to
// proxy, may be sufficient to keep most traffic from going through the proxy.

type Rule interface {
	Check(ip net.IP, port int) bool
}

type oneIPRule struct {
	ip net.IP
}

func (r oneIPRule) Check(ip net.IP, port int) bool {
	return r.ip.Equal(ip)
}

func MakeOneIPRule(ip net.IP) Rule {
	return oneIPRule{ip}
}

type onePortRule struct {
	port int
}

func (r onePortRule) Check(ip net.IP, port int) bool {
	return r.port == port
}

func MakeOnePortRule(port int) Rule {
	return onePortRule{port}
}

// tcpDialer switches between two dialers based on an IPRule
type tcpDialer struct {
	rule                         Rule
	specialDialer, defaultDialer core.TCPDialer
}

func NewTCPDialer(rule Rule, specialDialer, defaultDialer core.TCPDialer) core.TCPDialer {
	return tcpDialer{
		rule:          rule,
		specialDialer: specialDialer,
		defaultDialer: defaultDialer,
	}
}

func (h tcpDialer) DialTCP(target *net.TCPAddr) (core.DoubleStream, error) {
	// DNS override
	if h.rule.Check(target.IP, target.Port) {
		return h.specialDialer.DialTCP(target)
	}
	return h.defaultDialer.DialTCP(target)
}

type udpListener struct {
	rule                             Rule
	specialListener, defaultListener core.UDPListener
}

func MakeUDPListener(rule Rule, specialListener, defaultListener core.UDPListener) core.UDPListener {
	return udpListener{
		rule:            rule,
		specialListener: specialListener,
		defaultListener: defaultListener,
	}
}

func (l udpListener) ListenUDP(reverse core.ReverseUDPConn) (net.PacketConn, error) {
	defaultConn, err := l.defaultListener.ListenUDP(reverse)
	if err != nil {
		return nil, err
	}
	conn := &packetConnWrapper{
		PacketConn:      defaultConn,
		specialListener: l.specialListener,
		rule:            l.rule,
	}
	conn.reverse = &reverseConnWrapper{reverse, conn}
	return conn, nil
}

type reverseConnWrapper struct {
	core.ReverseUDPConn
	conn *packetConnWrapper
}

func (r *reverseConnWrapper) Close() error {
	r.conn.onReverseClosed()
	return r.ReverseUDPConn.Close()
}

type packetConnWrapper struct {
	net.PacketConn
	reverse         *reverseConnWrapper
	specialListener core.UDPListener
	rule            Rule
	specialConn     net.PacketConn
	usedDefault     int32
}

func (c *packetConnWrapper) WriteTo(p []byte, addr net.Addr) (int, error) {
	udpAddr, err := core.UDPAddr(addr)
	if err != nil {
		return 0, err
	}
	if !c.rule.Check(udpAddr.IP, udpAddr.Port) {
		atomic.StoreInt32(&c.usedDefault, 1)
		return c.PacketConn.WriteTo(p, udpAddr)
	}
	if c.specialConn == nil {
		if c.specialConn, err = c.specialListener.ListenUDP(c.reverse); err != nil {
			return 0, err
		}
		go core.CopyDownstream(c.reverse, c.specialConn)
	}
	return c.specialConn.WriteTo(p, udpAddr)
}

func (c *packetConnWrapper) onReverseClosed() {
	if atomic.LoadInt32(&c.usedDefault) == 0 {
		c.Close()
	}
}

type netsRule struct {
	nets []net.IPNet
}

func (r netsRule) Check(ip net.IP, port int) bool {
	for _, net := range r.nets {
		if net.Contains(ip) {
			log.Printf("IP Match: %s", ip.String())
			return true
		}
	}
	return false
}

func MakeIPNetsRule(nets []net.IPNet) Rule {
	return netsRule{nets}
}
