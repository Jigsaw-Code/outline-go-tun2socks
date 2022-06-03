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

package outline

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"github.com/Jigsaw-Code/outline-go-tun2socks/dns"
	"github.com/Jigsaw-Code/outline-go-tun2socks/intra/doh"
	oss "github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	"github.com/Jigsaw-Code/outline-go-tun2socks/split"
	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

// Tunnel represents a tunnel from a TUN device to a server.
type Tunnel interface {
	tunnel.Tunnel

	// UpdateUDPSupport determines if UDP is supported following a network connectivity change.
	// Sets the tunnel's UDP connection handler accordingly, falling back to DNS over TCP if UDP is not supported.
	// Returns whether UDP proxying is supported in the new network.
	UpdateUDPSupport() bool
}

type outlinetunnel struct {
	tunnel.Tunnel
	host         string
	port         int
	password     string
	cipher       string
	isUDPEnabled bool // Whether the tunnel supports proxying UDP.
}

func MakeSplitDNS(localDNS dns.Transport, pattern string) (dns.Transport, error) {
	rule, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	transport, err := doh.NewTransport("https://1.1.1.1/dns-query", nil, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return dns.MakeSplitDNS(
		rule,
		localDNS,
		transport,
	), nil
}

// NewTunnel connects a tunnel to a Shadowsocks proxy server and returns an `outline.Tunnel`.
//
// `host` is the IP or domain of the Shadowsocks proxy.
// `port` is the port of the Shadowsocks proxy.
// `password` is the password of the Shadowsocks proxy.
// `cipher` is the encryption cipher used by the Shadowsocks proxy.
// `isUDPEnabled` indicates if the Shadowsocks proxy and the network support proxying UDP traffic.
// `fakedns` is the IP address of the virtual DNS server
// `resolver` is the DNS resolver to use
// `ipNets` is a comma-separated list of CIDR networks that should be resolved outside of the proxy.
// `tunWriter` is used to output packets back to the TUN device.  OutlineTunnel.Disconnect() will close `tunWriter`.
func NewTunnel(host string, port int, password, cipher string, isUDPEnabled bool, resolver dns.Transport, ipNets string, endpoint stack.LinkEndpoint) (Tunnel, error) {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return nil, fmt.Errorf("Invalid Shadowsocks proxy parameters: %v", err.Error())
	}
	t := &outlinetunnel{nil, host, port, password, cipher, isUDPEnabled}
	dialer := oss.Dialer{Client: client}

	dnsRule := split.MakeOneIPRule(net.IP{10, 0, 85, 3})

	var ipNetParsed []net.IPNet
	if len(ipNets) > 0 {
		ipNetStrings := strings.Split(ipNets, ",")
		ipNetParsed := make([]net.IPNet, len(ipNetStrings))
		for i, s := range ipNetStrings {
			_, net, err := net.ParseCIDR(s)
			if err != nil {
				return nil, err
			}
			ipNetParsed[i] = *net
		}
	}
	ipRule := split.MakeIPNetsRule(ipNetParsed)

	dnsDialer := dns.NewTCPDialer()
	dnsDialer.SetDNS(resolver)
	tcp := split.NewTCPDialer(
		dnsRule,
		dnsDialer,
		split.NewTCPDialer(
			ipRule,
			core.MakeTCPDialer(nil), // FIXME: No escape route!
			dialer,
		),
	)

	dnsListener := dns.NewUDPListener()
	dnsListener.SetDNS(resolver)
	udp := split.MakeUDPListener(
		dnsRule,
		dnsListener,
		split.MakeUDPListener(
			ipRule,
			core.DefaultUDPListener{}, // FIXME: No escape route!
			dialer,
		),
	)

	t.Tunnel, err = tunnel.MakeTunnel(endpoint, tcp, udp, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *outlinetunnel) UpdateUDPSupport() bool {
	client, err := shadowsocks.NewClient(t.host, t.port, t.password, t.cipher)
	if err != nil {
		return false
	}
	isUDPEnabled := oss.CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("1.1.1.1:53", "udp")) == nil
	if t.isUDPEnabled != isUDPEnabled {
		t.isUDPEnabled = isUDPEnabled
		// FIXME: swap UDP handler
	}
	return isUDPEnabled
}
