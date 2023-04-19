// Copyright 2022 The Outline Authors
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

package shadowsocks

import (
	"fmt"
	"net"

	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

func NewShadowsocksConn(host string, port int, cipherName, password string, prefix []byte) (*client.StreamDialer, *transport.PacketListener, error) {
	if err := validateConfig(host, port, cipherName, password); err != nil {
		return nil, nil, fmt.Errorf("invalid shadowsocks configuration: %w", err)
	}

	// TODO: consider using net.LookupIP to get a list of IPs, and add logic for optimal selection.
	proxyIP, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve proxy address: %w", err)
	}

	proxyTCPEndpoint := transport.TCPEndpoint{RemoteAddr: net.TCPAddr{IP: proxyIP.IP, Port: port}}
	proxyUDPEndpoint := transport.UDPEndpoint{RemoteAddr: net.UDPAddr{IP: proxyIP.IP, Port: port}}

	cipher, err := shadowsocks.NewCipher(cipherName, password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Shadowsocks cipher: %w", err)
	}

	streamDialer, err := client.NewShadowsocksStreamDialer(proxyTCPEndpoint, cipher)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create StreamDialer: %w", err)
	}
	if len(prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(prefix))
		streamDialer.SetTCPSaltGenerator(client.NewPrefixSaltGenerator(prefix))
	}

	packetListener, err := client.NewShadowsocksPacketListener(proxyUDPEndpoint, cipher)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create PacketListener: %w", err)
	}

	return &streamDialer, &packetListener, nil
}

// validateConfig validates whether a Shadowsocks server configuration is valid
// (it won't do any connectivity tests)
//
// Returns nil if it is valid; or an error message.
func validateConfig(host string, port int, cipher, password string) error {
	if len(host) == 0 {
		return fmt.Errorf("must provide a host name or IP address")
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be within range [1..65535]")
	}
	if len(cipher) == 0 {
		return fmt.Errorf("must provide an encryption cipher method")
	}
	if len(password) == 0 {
		return fmt.Errorf("must provide a password")
	}
	return nil
}
