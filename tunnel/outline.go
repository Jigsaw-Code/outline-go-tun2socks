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

package tunnel

import (
	"errors"
	"io"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/dnsfallback"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
)

// Tunnel represents a tunnel from a TUN device to a server.
type OutlineTunnel interface {
	Tunnel
	// SetUDPEnabled indicates whether the tunnel and/or the network support UDP traffic.
	SetUDPEnabled(isUDPEnabled bool)
}

type outlinetunnel struct {
	*tunnel
	host         string
	port         uint16
	isUDPEnabled bool
}

// NewTunnel connects a tunnel to a SOCKS5 server and returns a `Tunnel` object.
//
// `host` is the IP or domain of the SOCKS server.
// `port` is the port of the SOCKS server.
// `isUDPEnabled` indicates if the SOCKS server and the network support proxying UDP traffic.
// `tunWriter` is used to output packets back to the TUN device.
func NewTunnel(host string, port uint16, isUDPEnabled bool, tunWriter io.WriteCloser) (OutlineTunnel, error) {
	if host == "" || tunWriter == nil {
		return nil, errors.New("Must provide a valid host address, and TUN writer")
	}
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunWriter.Write(data)
	})
	base := &tunnel{tunWriter, core.NewLWIPStack(), true}
	t := &outlinetunnel{base, host, port, isUDPEnabled}
	t.registerConnectionHandlers()
	return t, nil
}

func (t *outlinetunnel) SetUDPEnabled(isUDPEnabled bool) {
	if t.isUDPEnabled == isUDPEnabled {
		return
	}
	t.isUDPEnabled = isUDPEnabled
	t.lwipStack.Close() // Close exisiting connections to avoid using the previous handlers.
	t.registerConnectionHandlers()
}

// Registers UDP and TCP SOCKS connection handlers to the tunnel's host and port.
// Registers a DNS/TCP fallback UDP handler when UDP is disabled.
func (t *outlinetunnel) registerConnectionHandlers() {
	var udpHandler core.UDPConnHandler
	if t.isUDPEnabled {
		udpHandler = socks.NewUDPHandler(
			t.host, t.port, 30*time.Second, nil, nil)
	} else {
		udpHandler = dnsfallback.NewUDPHandler()
	}
	core.RegisterTCPConnHandler(socks.NewTCPHandler(t.host, t.port, nil))
	core.RegisterUDPConnHandler(udpHandler)
}
