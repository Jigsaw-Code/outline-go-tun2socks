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
	"net"
	"os"
	"strings"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/intra/doh"
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// Listener receives usage statistics when a UDP or TCP socket is closed,
// or a DNS query is completed.
type Listener interface {
	UDPListener
	TCPListener
	doh.Listener
}

// Tunnel represents an Intra session.
type Tunnel interface {
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
	tcp TCPDialer
	udp UDPConnListener
	dns doh.Transport
}

// NewIntraTunnel creates a connected Intra session.
//
// `fakedns` is the DNS server (IP and port) that will be used by apps on the TUN device.
//    This will normally be a reserved or remote IP address, port 53.
// `udpdns` and `tcpdns` are the actual location of the DNS server in use.
//    These will normally be localhost with a high-numbered port.
// `dohdns` is the initial DOH transport.
// `endpoint` is the TUN device
// `dialer` and `config` will be used for all network activity.
// `listener` will be notified at the completion of every tunneled socket.
func NewTunnel(fakedns string, dohdns doh.Transport, endpoint stack.LinkEndpoint, dialer *net.Dialer, config *net.ListenConfig, listener Listener) (Tunnel, error) {
	tcp, udp, err := getConnectionHandlers(fakedns, dialer, config, listener)
	if err != nil {
		return nil, err
	}
	// RFC 5382 REQ-5 requires a timeout no shorter than 2 hours and 4 minutes.
	timeout, _ := time.ParseDuration("2h4m")

	base, err := tunnel.MakeTunnel(endpoint, tcp, udp, timeout)
	if err != nil {
		return nil, err
	}
	t := &intratunnel{
		Tunnel: base,
		tcp:    tcp,
		udp:    udp,
	}
	t.SetDNS(dohdns)
	return t, nil
}

// Registers Intra's custom UDP and TCP connection handlers to the tun2socks core.
func getConnectionHandlers(fakedns string, dialer *net.Dialer, config *net.ListenConfig, listener Listener) (TCPDialer, UDPConnListener, error) {
	udpfakedns, err := net.ResolveUDPAddr("udp", fakedns)
	if err != nil {
		return nil, nil, err
	}
	udp := NewUDPConnListener(*udpfakedns, config, listener)
	tcpfakedns, err := net.ResolveTCPAddr("tcp", fakedns)
	if err != nil {
		return nil, nil, err
	}
	tcp := NewTCPDialer(*tcpfakedns, dialer, listener)
	return tcp, udp, nil
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
