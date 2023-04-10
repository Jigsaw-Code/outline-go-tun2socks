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
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/internal/utf8"
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/neterrors"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

// A client object that can be used to connect to a remote Shadowsocks proxy.
type Client = outline.Client

// NewClient creates a new Shadowsocks client from a non-nil configuration.
//
// [Deprecated] Please use NewClientFromJSON or NewClientFromParameters.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("Shadowsocks configuration is required")
	}
	return newClient(config.Host, config.Port, config.CipherName, config.Password, config.Prefix)
}

// NewClientFromJSON creates a new Shadowsocks client from a JSON formatted configuration.
func NewClientFromJSON(configJSON string) (*Client, error) {
	config, err := parseConfigFromJSON(configJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Shadowsocks configuration JSON: %w", err)
	}
	return NewClientFromParameters(config.Host, int(config.Port), config.Method, config.Password, config.Prefix)
}

// NewClientFromParameters creates a new Shadowsocks client from the parameters.
//
//   - host specifies the hostname/IP of the Shadowsocks server
//   - port specifies the port number of the Shadowsocks server
//   - cipher and password specifies the encryption strategy
//   - prefix is an optional string, it specifies the salt prefix used in the
//     beginning of the session. The should be a UTF-8 encoded string with each
//     codepoint representing a single byte (0x00 ~ 0xFF) in the salt prefix.
func NewClientFromParameters(host string, port int, cipher, password string, prefix string) (*Client, error) {
	var prefixBytes []byte = nil
	if len(prefix) > 0 {
		if p, err := utf8.DecodeCodepointsToBytes(prefix); err != nil {
			return nil, fmt.Errorf("Failed to parse prefix string: %w", err)
		} else {
			prefixBytes = p
		}
	}
	return newClient(host, port, cipher, password, prefixBytes)
}

func newClient(host string, port int, cipherName, password string, prefix []byte) (*Client, error) {
	if err := validateConfig(host, port, cipherName, password); err != nil {
		return nil, fmt.Errorf("Invalid Shadowsocks configuration: %w", err)
	}

	// TODO: consider using net.LookupIP to get a list of IPs, and add logic for optimal selection.
	proxyIP, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve proxy address: %w", err)
	}
	proxyTCPEndpoint := transport.TCPEndpoint{RemoteAddr: net.TCPAddr{IP: proxyIP.IP, Port: port}}
	proxyUDPEndpoint := transport.UDPEndpoint{RemoteAddr: net.UDPAddr{IP: proxyIP.IP, Port: port}}

	cipher, err := shadowsocks.NewCipher(cipherName, password)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Shadowsocks cipher: %w", err)
	}

	streamDialer, err := client.NewShadowsocksStreamDialer(proxyTCPEndpoint, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create StreamDialer: %w", err)
	}
	if len(prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(prefix))
		streamDialer.SetTCPSaltGenerator(client.NewPrefixSaltGenerator(prefix))
	}

	packetListener, err := client.NewShadowsocksPacketListener(proxyUDPEndpoint, cipher)
	if err != nil {
		return nil, fmt.Errorf("Failed to create PacketListener: %w", err)
	}

	return &outline.Client{StreamDialer: streamDialer, PacketListener: packetListener}, nil
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
