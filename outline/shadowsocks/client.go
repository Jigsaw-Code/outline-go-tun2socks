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

// This package provides support of Shadowsocks client and the configuration
// that can be used by Outline Client.
//
// All data structures and functions will also be exposed as libraries that
// non-golang callers can use (for example, C/Java/Objective-C).
package shadowsocks

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/outline"
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/connectivity"
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/neterrors"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

// [Exported] A Shadowsocks client that can be used by Outline-Apps
type Client = outline.Client

// [Exported] Create a new Shadowsocks client from a non-nil configuration.
func NewClient(config *Config) (*Client, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("Invalid Shadowsocks configuration: %w", err)
	}

	// TODO: consider using net.LookupIP to get a list of IPs, and add logic for optimal selection.
	proxyIP, err := net.ResolveIPAddr("ip", config.Host)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve proxy address: %w", err)
	}
	proxyTCPEndpoint := transport.TCPEndpoint{RemoteAddr: net.TCPAddr{IP: proxyIP.IP, Port: config.Port}}
	proxyUDPEndpoint := transport.UDPEndpoint{RemoteAddr: net.UDPAddr{IP: proxyIP.IP, Port: config.Port}}

	cipher, err := shadowsocks.NewCipher(config.CipherName, config.Password)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Shadowsocks cipher: %w", err)
	}

	streamDialer, err := client.NewShadowsocksStreamDialer(proxyTCPEndpoint, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create StreamDialer: %w", err)
	}
	if len(config.Prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(config.Prefix))
		streamDialer.SetTCPSaltGenerator(client.NewPrefixSaltGenerator(config.Prefix))
	}

	packetListener, err := client.NewShadowsocksPacketListener(proxyUDPEndpoint, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create PacketListener: %w", err)
	}

	return &outline.Client{StreamDialer: streamDialer, PacketListener: packetListener}, nil
}

// Validates a Shadowsocks server configuration object, returns nil if it is acceptable.
func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if len(config.Host) == 0 {
		return fmt.Errorf("must provide a host name or IP address")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("port must be within range [1..65535]")
	}
	if len(config.Password) == 0 {
		return fmt.Errorf("must provide a password")
	}
	if len(config.CipherName) == 0 {
		return fmt.Errorf("must provide an encryption cipher method")
	}
	return nil
}

const reachabilityTimeout = 10 * time.Second

// CheckConnectivity determines whether the Shadowsocks proxy can relay TCP and UDP traffic under
// the current network. Parallelizes the execution of TCP and UDP checks, selects the appropriate
// error code to return accounting for transient network failures.
// Returns an error if an unexpected error ocurrs.
func CheckConnectivity(client *Client) (neterrors.Error, error) {
	return connectivity.CheckConnectivity(client)
}

// CheckServerReachable determines whether the server at `host:port` is reachable over TCP.
// Returns an error if the server is unreachable.
func CheckServerReachable(host string, port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), reachabilityTimeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
