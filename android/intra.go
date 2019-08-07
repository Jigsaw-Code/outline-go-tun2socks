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
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
)

// IntraTunnel embeds the tun2socks.Tunnel interface so it gets exported by gobind.
// Intra does not need any methods beyond the basic Tunnel interface.
type IntraTunnel interface {
	tunnel.Tunnel
}

// ConnectIntraTunnel reads packets from a TUN device and applies the Intra routing
// rules.  Currently, this only consists of redirecting DNS packets to a specified
// server; all other data flows directly to its destination.
//
// `fakedns` is the DNS server that the system believes it is using, in "host:port" style.
//   The port is normally 53.
// `udpdns` and `tcpdns` are the location of the actual DNS server being used.  For DNS
//   tunneling in Intra, these are typically high-numbered ports on localhost.
//
// Throws an exception if the TUN file descriptor cannot be opened, or if the tunnel fails to
// connect.
func ConnectIntraTunnel(fd int, fakedns, udpdns, tcpdns string, alwaysSplitHTTPS bool, listener tunnel.IntraListener) (IntraTunnel, error) {
	tun, err := makeTunFile(fd)
	if err != nil {
		return nil, err
	}
	tunnel, err := tunnel.NewIntraTunnel(fakedns, udpdns, tcpdns, tun, alwaysSplitHTTPS, listener)
	if err != nil {
		return nil, err
	}
	go processInputPackets(tunnel, tun)
	return tunnel, nil
}
