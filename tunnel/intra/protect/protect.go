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

package protect

import (
	"context"
	"net"
	"sync/atomic"
	"syscall"
)

// Protector is a wrapper for Android's VpnService.protect().
type Protector interface {
	// Protect a socket, i.e. exclude it from the VPN.
	// This is needed in order to avoid routing loops for the VPN's own sockets.
	Protect(socket int32) bool
}

func makeRawConnControl(p Protector) (func(uintptr)) {
	return func(fd uintptr) {
		if p != nil {
			if !p.Protect(int32(fd)) {
				panic("Failed to protect socket")
			}
		}
	}
}

func makeDialerControl(p Protector) (func(string, string, syscall.RawConn) error) {
	rawConnControl := makeRawConnControl(p)
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(rawConnControl)
	}
}

func dialer(p Protector) *net.Dialer {
	return &net.Dialer{
		Control: makeDialerControl(p),
	}
}

func dialTCP(p Protector, raddr *net.TCPAddr) (*net.TCPConn, error) {
	conn, err := dialer(p).Dial(raddr.Network(), raddr.String())
	if err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), nil
}

func listenUDP(p Protector, laddr *net.UDPAddr) (*net.UDPConn, error) {
	conn, err := net.ListenUDP(laddr.Network(), laddr)
	if err != nil {
		return nil, err
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	raw.Control(makeRawConnControl(p))
	return conn, nil
}

func lookupIPAddr(p Protector, host string) ([]net.IPAddr, error) {
	resolver := &net.Resolver {
		PreferGo: true,
		Dial: dialer(p).DialContext,
	}
	return resolver.LookupIPAddr(context.Background(), host)
}

// There can only be one VPN active at a time, so there can only be one
// active Protector.  Racing reads and writes to this value should be rare,
// but the atomic value might become necessary if a socket is dialing while
// the VPN service is being restarted.
var singleton atomic.Value

// SetProtector sets the active Protector to this value.
// `p` must not be nil.
func SetProtector(p Protector) {
	singleton.Store(p)
}

func getProtector() Protector {
	p := singleton.Load()
	if p == nil {
		return nil
	}
	return p.(Protector)
}

// These functions are public static interface of this package.  They are structured
// as trivial wrappers around private functions in order to enable unit testing.
// TODO: Avoid allocating a new Dialer on every call.

// Dialer returns a new net.Dialer that produces protected sockets.  Callers own
// the Dialer and are free to modify any field other than Dialer.Control.
func Dialer() *net.Dialer {
	return dialer(getProtector())
}

// DialTCP is a replacement for net.DialTCP that produces protected sockets.
func DialTCP(raddr *net.TCPAddr) (*net.TCPConn, error) {
	return dialTCP(getProtector(), raddr)
}

// ListenUDP is a replacement for net.ListenUDP that produces protected sockets.
func ListenUDP(laddr *net.UDPAddr) (*net.UDPConn, error) {
	return listenUDP(getProtector(), laddr)
}

// LookupIPAddr is an IP lookup function that sends DNS queries over a
// protected socket.
func LookupIPAddr(host string) ([]net.IPAddr, error) {
	return lookupIPAddr(getProtector(), host)
}
