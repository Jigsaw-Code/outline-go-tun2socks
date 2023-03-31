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
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/outline"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport/shadowsocks/client"
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
// It's used by the connectivity test and the tun2socks handlers.
type Client struct {
	transport.StreamDialer
	transport.PacketListener
}

// NewClient provides a gobind-compatible wrapper for [client.NewClient].
func NewClient(config *Config) (*Client, error) {
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

	return &Client{streamDialer, packetListener}, nil
}

// Outline error codes. Must be kept in sync with definitions in outline-client/cordova-plugin-outline/outlinePlugin.js
const (
	NoError                     = 0
	Unexpected                  = 1
	NoVPNPermissions            = 2
	AuthenticationFailure       = 3
	UDPConnectivity             = 4
	Unreachable                 = 5
	VpnStartFailure             = 6
	IllegalConfiguration        = 7
	ShadowsocksStartFailure     = 8
	ConfigureSystemProxyFailure = 9
	NoAdminPermissions          = 10
	UnsupportedRoutingTable     = 11
	SystemMisconfigured         = 12
)

const reachabilityTimeout = 10 * time.Second

// CheckConnectivity determines whether the Shadowsocks proxy can relay TCP and UDP traffic under
// the current network. Parallelizes the execution of TCP and UDP checks, selects the appropriate
// error code to return accounting for transient network failures.
// Returns an error if an unexpected error ocurrs.
func CheckConnectivity(client *Client) (int, error) {
	// Start asynchronous UDP support check.
	udpChan := make(chan error)
	go func() {
		resolverAddr := &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 53}
		udpChan <- outline.CheckUDPConnectivityWithDNS(client, resolverAddr)
	}()
	// Check whether the proxy is reachable and that the client is able to authenticate to the proxy
	tcpErr := outline.CheckTCPConnectivityWithHTTP(client, "http://example.com")
	if tcpErr == nil {
		udpErr := <-udpChan
		if udpErr == nil {
			return NoError, nil
		}
		return UDPConnectivity, nil
	}
	var authErr *outline.AuthenticationError
	var reachabilityErr *outline.ReachabilityError
	if errors.As(tcpErr, &authErr) {
		return AuthenticationFailure, nil
	} else if errors.As(tcpErr, &reachabilityErr) {
		return Unreachable, nil
	}
	// The error is not related to the connectivity checks.
	return Unexpected, tcpErr
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
