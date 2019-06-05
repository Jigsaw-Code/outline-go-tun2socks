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

package tunnel

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel/intra"
	"github.com/eycorsican/go-tun2socks/core"
)

type IntraListener interface {
	intra.UDPListener
	intra.TCPListener
}

type intratunnel struct {
	*tunnel
	fakedns  net.Addr
	udpdns   net.Addr
	tcpdns   net.Addr
	listener IntraListener
}

// NewIntraTunnel creates a connected Intra session.
//
// `fakedns` is the DNS server (IP and port) that will be used by apps on the TUN device.
//    This will normally be a reserved or remote IP address, port 53.
// `udpdns` and `tcpdns` are the actual location of the DNS server in use.
//    These will normally be localhost with a high-numbered port.
func NewIntraTunnel(fakedns, udpdns, tcpdns string, tunWriter io.WriteCloser, listener IntraListener) (Tunnel, error) {
	fakednsipaddr, err := net.ResolveUDPAddr("udp", fakedns)
	if err != nil {
		return nil, err
	}
	tcpdnsipaddr, err := net.ResolveTCPAddr("tcp", tcpdns)
	if err != nil {
		return nil, err
	}
	udpdnsipaddr, err := net.ResolveUDPAddr("udp", udpdns)
	if err != nil {
		return nil, err
	}
	if tunWriter == nil {
		return nil, errors.New("Must provide a valid TUN writer")
	}
	core.RegisterOutputFn(tunWriter.Write)
	base := &tunnel{tunWriter, core.NewLWIPStack(), true}
	s := &intratunnel{tunnel: base, fakedns: fakednsipaddr, udpdns: udpdnsipaddr, tcpdns: tcpdnsipaddr, listener: listener}
	s.registerConnectionHandlers()
	return s, nil
}

// Registers Intra's custom UDP and TCP connection handlers to the tun2socks core.
func (t *intratunnel) registerConnectionHandlers() {
	// RFC 5382 REQ-5 requires a timeout no shorter than 2 hours and 4 minutes.
	timeout, _ := time.ParseDuration("2h4m")

	core.RegisterUDPConnHandler(intra.NewUDPHandler(t.fakedns, t.udpdns, timeout, t.listener))
	core.RegisterTCPConnHandler(intra.NewTCPHandler(t.fakedns, t.tcpdns, t.listener))
}
