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

package tun2socks

import (
	"fmt"
	"math"
	"runtime/debug"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
	"github.com/eycorsican/go-tun2socks/common/log"
)

func init() {
	// Conserve memory by increasing garbage collection frequency.
	debug.SetGCPercent(10)
	log.SetLevel(log.WARN)
}

// OutlineTunnel embeds the tun2socks.OutlineTunnel interface so it gets exported by gobind.
type OutlineTunnel interface {
	tunnel.OutlineTunnel
}

// ConnectShadowsocksTunnel reads packets from a TUN device and routes it to a Shadowsocks proxy server.
// Returns an OutlineTunnel instance and does *not* take ownership of the TUN file descriptor; the
// caller is responsible for closing after OutlineTunnel disconnects.
//
// `fd` is the TUN device.  The OutlineTunnel acquires an additional reference to it, which
//     is released by OutlineTunnel.Disconnect(), so the caller must close `fd` _and_ call
//     Disconnect() in order to close the TUN device.
// `host` is  IP address of the Shadowsocks proxy server.
// `port` is the port of the Shadowsocks proxy server.
// `password` is the password of the Shadowsocks proxy.
// `cipher` is the encryption cipher the Shadowsocks proxy.
// `isUDPEnabled` indicates whether the tunnel and/or network enable UDP proxying.
//
// Throws an exception if the TUN file descriptor cannot be opened, or if the tunnel fails to
// connect.
func ConnectShadowsocksTunnel(fd int, host string, port int, password, cipher string, isUDPEnabled bool) (OutlineTunnel, error) {
	if port <= 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("Invalid port number: %v", port)
	}
	tun, err := tunnel.MakeTunFile(fd)
	if err != nil {
		return nil, err
	}
	t, err := tunnel.NewOutlineTunnel(host, port, password, cipher, isUDPEnabled, tun)
	if err != nil {
		return nil, err
	}
	go tunnel.ProcessInputPackets(t, tun)
	return t, nil
}
