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

package tun2socks

import (
	"errors"
	"io"
	"time"

	"github.com/eycorsican/go-tun2socks/common/dns/cache"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/dnsfallback"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
)

// Tunnel represents a tunnel from a TUN device to a server.
type Tunnel interface {
	// IsConnected indicates whether the tunnel is in a connected state.
	IsConnected() bool
	// SetUDPEnabled indicates whether the tunnel and/or the network support UDP traffic.
	SetUDPEnabled(isUDPEnabled bool)
	// Disconnect disconnects the tunnel.
	Disconnect()
	// Write writes input data to the TUN interface.
	Write(data []byte) (int, error)
}

type tunnel struct {
	host         string
	port         uint16
	isConnected  bool
	isUDPEnabled bool
	lwipStack    core.LWIPStack
	tunWriter    io.WriteCloser
}

// NewTunnel connects a tunnel to a SOCKS5 server and returns a `Tunnel` object.
//
// `host` is the IP or domain of the SOCKS server.
// `port` is the port of the SOCKS server.
// `isUDPEnabled` indicates if the SOCKS server and the network support proxying UDP traffic.
// `tunWriter` is used to output packets back to the TUN device.
func NewTunnel(host string, port uint16, isUDPEnabled bool, tunWriter io.WriteCloser) (Tunnel, error) {
	if host == "" || tunWriter == nil {
		return nil, errors.New("Must provide a valid host address, and TUN writer")
	}
	var lwipStack = core.NewLWIPStack()
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunWriter.Write(data)
	})
	t := &tunnel{host: host, port: port, isUDPEnabled: isUDPEnabled, lwipStack: lwipStack,
		tunWriter: tunWriter, isConnected: true}
	t.registerConnectionHandlers()
	return t, nil
}

func (t *tunnel) IsConnected() bool {
	return t.isConnected
}

func (t *tunnel) SetUDPEnabled(isUDPEnabled bool) {
	if t.isUDPEnabled == isUDPEnabled {
		return
	}
	t.isUDPEnabled = isUDPEnabled
	t.lwipStack.Close() // Close exisiting connections to avoid using the previous handlers.
	t.registerConnectionHandlers()
}

func (t *tunnel) Disconnect() {
	if !t.isConnected {
		return
	}
	t.isConnected = false
	t.tunWriter.Close()
	t.lwipStack.Close()
}

func (t *tunnel) Write(data []byte) (int, error) {
	if !t.isConnected {
		return 0, errors.New("Failed to write, network stack closed")
	}
	return t.lwipStack.Write(data)
}

// Registers UDP and TCP SOCKS connection handlers to the tunnel's host and port.
// Registers a DNS/TCP fallback UDP handler when UDP is disabled.
func (t *tunnel) registerConnectionHandlers() {
	var udpHandler core.UDPConnHandler
	if t.isUDPEnabled {
		udpHandler = socks.NewUDPHandler(
			t.host, t.port, 30*time.Second, cache.NewSimpleDnsCache(), nil)
	} else {
		udpHandler = dnsfallback.NewUDPHandler()
	}
	core.RegisterTCPConnHandler(socks.NewTCPHandler(t.host, t.port, nil))
	core.RegisterUDPConnHandler(udpHandler)
}
