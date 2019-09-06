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
	"runtime/debug"
	"strings"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel/intra"
	"github.com/eycorsican/go-tun2socks/common/log"
)

func init() {
	// Conserve memory by increasing garbage collection frequency.
	debug.SetGCPercent(10)
	log.SetLevel(log.WARN)
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
func ConnectIntraTunnel(fd int, fakedns, udpdns, tcpdns string, alwaysSplitHTTPS bool, listener tunnel.IntraListener) (tunnel.IntraTunnel, error) {
	tun, err := tunnel.MakeTunFile(fd)
	if err != nil {
		return nil, err
	}
	t, err := tunnel.NewIntraTunnel(fakedns, udpdns, tcpdns, tun, alwaysSplitHTTPS, listener)
	if err != nil {
		return nil, err
	}
	go tunnel.ProcessInputPackets(t, tun)
	return t, nil
}

// NewDoHTransport returns a DNSTransport that connects to the specified DoH server.
// `url` is the URL of a DoH server (no template, POST-only).  If it is nonempty, it
//   overrides `udpdns` and `tcpdns`.  `ips` is an optional comma-separated list of
//   IP addresses for the server.  (This wrapper is required because gomobile can't
//   make bindings for []string.)
func NewDoHTransport(url string, ips string, listener tunnel.IntraListener) (intra.DNSTransport, error) {
	split := []string{}
	if len(ips) > 0 {
		split = strings.Split(ips, ",")
	}
	return intra.NewDoHTransport(url, split, listener)
}
