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
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// Tunnel represents a session on a TUN device.
type Tunnel interface {
	// IsConnected indicates whether the tunnel is in a connected state.
	IsConnected() bool
	// Disconnect disconnects the tunnel.
	Disconnect()
}

func MakeTunnel(link stack.LinkEndpoint, dialer core.TCPDialer, listener core.UDPListener, timeout time.Duration) (*tunnel, error) {
	const nicID = 1
	netstack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		HandleLocal:        false, // false to force all traffic to be forwarded.
	})
	netstack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPSendBufferSizeRangeOption{
		Max: 2048, // Ideally this should probably be exactly one MSS
	})
	netstack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPReceiveBufferSizeRangeOption{
		Max: 18000, // 16KB (shadowsocks max) + 1 MTU should be plenty
	})
	if neterr := netstack.CreateNICWithOptions(nicID, link, stack.NICOptions{Disabled: true}); neterr != nil {
		return nil, errors.New(neterr.String())
	}
	// Route everything
	netstack.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv6EmptySubnet,
			NIC:         nicID,
		},
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         nicID,
		},
	})
	if err := netstack.SetSpoofing(1, true); err != nil {
		return nil, fmt.Errorf("failed to SetSpoofing: %v", err)
	}
	if neterr := netstack.SetPromiscuousMode(nicID, true); neterr != nil {
		return nil, errors.New(neterr.String())
	}

	tcpBridge := MakeTCPBridge(netstack, dialer)
	netstack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpBridge.HandlePacket)
	udpBridge := MakeUDPBridge(netstack, listener, timeout)
	netstack.SetTransportProtocolHandler(udp.ProtocolNumber, udpBridge.HandleOutboundPacket)

	if neterr := netstack.EnableNIC(nicID); neterr != nil {
		return nil, errors.New(neterr.String())
	}
	return &tunnel{netstack, link, true}, nil
}

type tunnel struct {
	netstack    *stack.Stack
	link        stack.LinkEndpoint
	isConnected bool
}

func (t *tunnel) IsConnected() bool {
	return t.isConnected
}

func (t *tunnel) Disconnect() {
	if !t.isConnected {
		return
	}
	t.isConnected = false
	t.netstack.Close()
}
