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

	"github.com/eycorsican/go-tun2socks/core"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel/intra"
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel/intra/doh"
)

// IntraListener receives usage statistics when a UDP or TCP socket is closed,
// or a DNS query is completed.
type IntraListener interface {
	intra.UDPListener
	intra.TCPListener
	doh.Listener
}

// IntraTunnel represents an Intra session.
type IntraTunnel interface {
	Tunnel
	// Get the DNSTransport (default: nil).
	GetDNS() doh.Transport
	// Set the DNSTransport.  Once set, the tunnel will send DNS queries to
	// this transport instead of forwarding them to `udpdns`/`tcpdns`.  The
	// transport can be changed at any time during operation, but must not be nil.
	SetDNS(doh.Transport)
	// When set to true, Intra will pre-emptively split all HTTPS connections.
	SetAlwaysSplitHTTPS(bool)
}

type intratunnel struct {
	*tunnel
	tcp intra.TCPHandler
	udp intra.UDPHandler
	dns doh.Transport
}

// NewIntraTunnel creates a connected Intra session.
//
// `fakedns` is the DNS server (IP and port) that will be used by apps on the TUN device.
//    This will normally be a reserved or remote IP address, port 53.
// `udpdns` and `tcpdns` are the actual location of the DNS server in use.
//    These will normally be localhost with a high-numbered port.
// `dohdns` is the initial DOH transport.
// TODO: Remove `udpdns` and `tcpdns` once DOH-in-Go is fully rolled out.
func NewIntraTunnel(fakedns, udpdns, tcpdns string, dohdns doh.Transport, tunWriter io.WriteCloser, listener IntraListener) (IntraTunnel, error) {
	if tunWriter == nil {
		return nil, errors.New("Must provide a valid TUN writer")
	}
	core.RegisterOutputFn(tunWriter.Write)
	base := &tunnel{tunWriter, core.NewLWIPStack(), true}
	t := &intratunnel{
		tunnel: base,
	}
	if err := t.registerConnectionHandlers(fakedns, udpdns, tcpdns, listener); err != nil {
		return nil, err
	}
	if dohdns != nil {
		t.SetDNS(dohdns)
	}
	return t, nil
}

// Registers Intra's custom UDP and TCP connection handlers to the tun2socks core.
func (t *intratunnel) registerConnectionHandlers(fakedns, udpdns, tcpdns string, listener IntraListener) error {
	// RFC 5382 REQ-5 requires a timeout no shorter than 2 hours and 4 minutes.
	timeout, _ := time.ParseDuration("2h4m")

	udpfakedns, err := net.ResolveUDPAddr("udp", fakedns)
	if err != nil {
		return err
	}
	udptruedns, err := net.ResolveUDPAddr("udp", udpdns)
	if err != nil {
		return err
	}
	t.udp = intra.NewUDPHandler(*udpfakedns, *udptruedns, timeout, listener)
	core.RegisterUDPConnHandler(t.udp)

	tcpfakedns, err := net.ResolveTCPAddr("tcp", fakedns)
	if err != nil {
		return err
	}
	tcptruedns, err := net.ResolveTCPAddr("tcp", tcpdns)
	if err != nil {
		return err
	}
	t.tcp = intra.NewTCPHandler(*tcpfakedns, *tcptruedns, listener)
	core.RegisterTCPConnHandler(t.tcp)
	return nil
}

func (t *intratunnel) SetDNS(dns doh.Transport) {
	t.dns = dns
	t.udp.SetDNS(dns)
	t.tcp.SetDNS(dns)
}

func (t *intratunnel) GetDNS() doh.Transport {
	return t.dns
}

func (t *intratunnel) SetAlwaysSplitHTTPS(s bool) {
	t.tcp.SetAlwaysSplitHTTPS(s)
}
