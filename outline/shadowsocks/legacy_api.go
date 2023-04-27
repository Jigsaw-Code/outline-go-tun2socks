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
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/internal/shadowsocks"
)

// Config represents a (legacy) shadowsocks server configuration. You can use
// NewClientFromJSON(string) instead.
//
// Deprecated: this object will be removed once we migrated from the old
// Outline Client logic.
type Config struct {
	Host       string
	Port       int
	Password   string
	CipherName string
	Prefix     []byte
}

// A client object that can be used to connect to a remote Shadowsocks proxy.
//
// Deprecated: Keep for backward compatibility only, please use outline.Client.
type Client outline.Client

// NewClient creates a new Shadowsocks client from a non-nil configuration.
//
// Deprecated: Keep for backward compatibility only.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("Shadowsocks configuration is required")
	}
	sd, pl, err := shadowsocks.NewTransport(config.Host, config.Port, config.CipherName, config.Password, config.Prefix)
	if err != nil {
		return nil, err
	}
	return &Client{sd, pl}, err
}

const reachabilityTimeout = 10 * time.Second

// CheckConnectivity determines whether the Shadowsocks proxy can relay TCP and UDP traffic under
// the current network. Parallelizes the execution of TCP and UDP checks, selects the appropriate
// error code to return accounting for transient network failures.
// Returns an error if an unexpected error ocurrs.
//
// Note: please make sure the return type is (int, error) for backward compatibility reason.
//
// Deprecated: Keep for backward compatibility only.
func CheckConnectivity(client *Client) (int, error) {
	netErr, err := connectivity.CheckConnectivity((*outline.Client)(client))
	return netErr.Number(), err
}

// CheckServerReachable determines whether the server at `host:port` is reachable over TCP.
// Returns an error if the server is unreachable.
//
// Deprecated: Keep for backward compatibility only.
func CheckServerReachable(host string, port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), reachabilityTimeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
