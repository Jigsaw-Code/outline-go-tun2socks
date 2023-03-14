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

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	ss "github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
	ss_client "github.com/Jigsaw-Code/outline-ss-server/shadowsocks/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

// Config represents a shadowsocks server configuration.
// Exported via gobind.
type Config struct {
	Host       string
	Port       int
	Password   string
	CipherName string
	Prefix     []byte
}

// Client provides a transparent container for [client.Client] that
// is exportable (as an opaque object) via gobind.
type Client struct {
	onet.StreamDialer
	onet.PacketListener
}

// NewClient provides a gobind-compatible wrapper for [client.NewClient].
func NewClient(config *Config) (*Client, error) {
	// TODO: consider using net.LookupIP to get a list of IPs, and add logic for optimal selection.
	proxyIP, err := net.ResolveIPAddr("ip", config.Host)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve proxy address: %w", err)
	}
	cipher, err := ss.NewCipher(config.CipherName, config.Password)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Shadowsocks cipher: %w", err)
	}
	streamDialer, err := ss_client.NewStreamDialer(proxyIP.String(), config.Port, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create StreamDialer: %w", err)
	}
	packetListener, err := ss_client.NewPacketListener(proxyIP.String(), config.Port, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create PacketListener: %w", err)
	}
	if len(config.Prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(config.Prefix))
		streamDialer.SetTCPSaltGenerator(ss_client.NewPrefixSaltGenerator(config.Prefix))
	}
	return &Client{streamDialer, packetListener}, nil
}
