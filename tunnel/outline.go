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
	"fmt"
	"io"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/dnsfallback"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

// OutlineTunnel represents a tunnel from a TUN device to a server.
type OutlineTunnel interface {
	Tunnel

	// UpdateUDPSupport determines if UDP is supported following a network connectivity change.
	// Sets the tunnel's UDP connection handler accordingly, falling back to DNS over TCP if UDP is not supported.
	// Returns whether UDP proxying is supported in the new network.
	UpdateUDPSupport() bool
}

type outlinetunnel struct {
	*tunnel
	host         string
	port         int
	password     string
	cipher       string
	isUDPEnabled bool // Whether the tunnel supports proxying UDP.
}

// NewOutlineTunnel connects a tunnel to a Shadowsocks proxy server and returns an `OutlineTunnel`.
//
// `host` is the IP or domain of the Shadowsocks proxy.
// `port` is the port of the Shadowsocks proxy.
// `password` is the password of the Shadowsocks proxy.
// `cipher` is the encryption cipher used by the Shadowsocks proxy.
// `isUDPEnabled` indicates if the Shadowsocks proxy and the network support proxying UDP traffic.
// `tunWriter` is used to output packets back to the TUN device.  OutlineTunnel.Disconnect() will close `tunWriter`.
func NewOutlineTunnel(host string, port int, password, cipher string, isUDPEnabled bool, tunWriter io.WriteCloser) (OutlineTunnel, error) {
	if tunWriter == nil {
		return nil, errors.New("Must provide a TUN writer")
	}
	_, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return nil, fmt.Errorf("Invalid Shadowsocks proxy parameters: %v", err.Error())
	}
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunWriter.Write(data)
	})
	base := &tunnel{tunWriter, core.NewLWIPStack(), true}
	t := &outlinetunnel{base, host, port, password, cipher, isUDPEnabled}
	t.registerConnectionHandlers()
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
		t.lwipStack.Close() // Close existing connections to avoid using the previous handlers.
		t.registerConnectionHandlers()
	}
	return isUDPEnabled
}

// Registers UDP and TCP Shadowsocks connection handlers to the tunnel's host and port.
// Registers a DNS/TCP fallback UDP handler when UDP is disabled.
func (t *outlinetunnel) registerConnectionHandlers() {
	var udpHandler core.UDPConnHandler
	if t.isUDPEnabled {
		udpHandler = oss.NewUDPHandler(t.host, t.port, t.password, t.cipher, 30*time.Second)
	} else {
		udpHandler = dnsfallback.NewUDPHandler()
	}
	core.RegisterTCPConnHandler(oss.NewTCPHandler(t.host, t.port, t.password, t.cipher))
	core.RegisterUDPConnHandler(udpHandler)
}
