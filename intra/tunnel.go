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

package intra

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/eycorsican/go-tun2socks/core"

	"github.com/Jigsaw-Code/outline-go-tun2socks/intra/doh"
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
)

// IntraListener receives usage statistics when a UDP or TCP socket is closed,
// or a DNS query is completed.
type IntraListener interface {
	UDPListener
	TCPListener
	doh.Listener
}

// IntraTunnel represents an Intra session.
type IntraTunnel interface {
	tunnel.Tunnel
	// Get the DNSTransport (default: nil).
	GetDNS() doh.Transport
	// Set the DNSTransport.  This method must be called before connecting the transport
	// to the TUN device.  The transport can be changed at any time during operation, but
	// must not be nil.
	SetDNS(doh.Transport)
	// When set to true, Intra will pre-emptively split all HTTPS connections.
	SetAlwaysSplitHTTPS(bool)
	// Enable reporting of SNIs that resulted in connection failures, using the
	// Choir library for privacy-preserving error reports.  `file` is the path
	// that Choir should use to store its persistent state, `suffix` is the
	// authoritative domain to which reports will be sent, and `country` is a
	// two-letter ISO country code for the user's current location.
	EnableSNIReporter(file, suffix, country string) error
}

type intratunnel struct {
	tunnel.Tunnel
	tcp TCPHandler
	udp UDPHandler
	dns doh.Transport
}

// NewIntraTunnel creates a connected Intra session.
//
// `fakedns` is the DNS server (IP and port) that will be used by apps on the TUN device.
//    This will normally be a reserved or remote IP address, port 53.
// `udpdns` and `tcpdns` are the actual location of the DNS server in use.
//    These will normally be localhost with a high-numbered port.
// `dohdns` is the initial DOH transport.
// `tunWriter` is the downstream VPN tunnel.  IntraTunnel.Disconnect() will close `tunWriter`.
// `dialer` and `config` will be used for all network activity.
// `listener` will be notified at the completion of every tunneled socket.
func NewIntraTunnel(fakedns string, dohdns doh.Transport, tunWriter io.WriteCloser, dialer *net.Dialer, config *net.ListenConfig, listener IntraListener) (IntraTunnel, error) {
	if tunWriter == nil {
		return nil, errors.New("Must provide a valid TUN writer")
	}
	core.RegisterOutputFn(tunWriter.Write)
	t := &intratunnel{
		Tunnel: tunnel.NewTunnel(tunWriter, core.NewLWIPStack()),
	}
	if err := t.registerConnectionHandlers(fakedns, dialer, config, listener); err != nil {
		return nil, err
	}
	t.SetDNS(dohdns)
	return t, nil
}

// Registers Intra's custom UDP and TCP connection handlers to the tun2socks core.
func (t *intratunnel) registerConnectionHandlers(fakedns string, dialer *net.Dialer, config *net.ListenConfig, listener IntraListener) error {
	// RFC 5382 REQ-5 requires a timeout no shorter than 2 hours and 4 minutes.
	timeout, _ := time.ParseDuration("2h4m")

	udpfakedns, err := net.ResolveUDPAddr("udp", fakedns)
	if err != nil {
		return err
	}
	t.udp = NewUDPHandler(*udpfakedns, timeout, config, listener)
	core.RegisterUDPConnHandler(t.udp)

	tcpfakedns, err := net.ResolveTCPAddr("tcp", fakedns)
	if err != nil {
		return err
	}
	t.tcp = NewTCPHandler(*tcpfakedns, dialer, listener)
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

func (t *intratunnel) EnableSNIReporter(filename, suffix, country string) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	return t.tcp.EnableSNIReporter(f, suffix, strings.ToLower(country))
}
