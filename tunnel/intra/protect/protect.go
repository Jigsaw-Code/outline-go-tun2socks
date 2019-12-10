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
	"net"
	"syscall"
)

// Protector is a wrapper for Android's VpnService.protect().
type Protector interface {
	// Protect a socket, i.e. exclude it from the VPN.
	// This is needed in order to avoid routing loops for the VPN's own sockets.
	Protect(socket int32) bool
}

func makeControl(p Protector) (func(string, string, syscall.RawConn) error) {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if !p.Protect(int32(fd)) {
				panic("Failed to protect socket")
			}
		})
	}
}

// MakeDialer creates a new Dialer.  Recipients can safely mutate
// any public field except Control and Resolver, which are both populated.
func MakeDialer(p Protector) *net.Dialer {
	if p == nil {
		return &net.Dialer{}
	}
	d := &net.Dialer{
		Control: makeControl(p),
	}
	d.Resolver = &net.Resolver{
		PreferGo: true,
		Dial: d.DialContext,
	}
	return d
}

// MakeListenConfig returns a new ListenConfig that creates protected
// listener sockets.
func MakeListenConfig(p Protector) *net.ListenConfig {
	if p == nil {
		return &net.ListenConfig{}
	}
	return &net.ListenConfig {
		Control: makeControl(p),
	}
}
