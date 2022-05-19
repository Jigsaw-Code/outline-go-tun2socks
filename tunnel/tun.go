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

package tunnel

import (
	"io"
	"log"

	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Import simple log for the side effect of making logs printable.
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const vpnMtu = 1500

type notifyWriter struct {
	tunWriter io.WriteCloser
	endpoint  *channel.Endpoint
	// Downstream packet header and body concatenation buffer.
	downstream [vpnMtu]byte
}

// Implements NotifyWriter
func (w *notifyWriter) WriteNotify() {
	if pkt := w.endpoint.Read(); pkt != nil {
		// Each downstream packet typically consists of two views:
		// a body view (arbitrary size) and a header view (IP + TCP).
		// We have to concatenate these into an IP packet before proceeding.
		// This copy could be avoided using unix.Writev, but it would require
		// access to the underlying file descriptor for tunWriter.
		data := w.downstream[:0]
		for _, view := range pkt.Views() {
			data = append(data, view...)
		}
		pkt.DecRef()
		if _, err := w.tunWriter.Write(data); err != nil {
			log.Printf("Downstream packet err=%v", err)
		}
	} else {
		log.Printf("Closing tunWriter")
		w.tunWriter.Close()
	}
}

type Endpoint struct {
	*channel.Endpoint
}

// Implements io.Writer.  pkt must be a complete IP packet.
func (e *Endpoint) Write(pkt []byte) (n int, err error) {
	n = len(pkt)
	// NewPacketBuffer takes ownership of the input.  TODO: Avoid copy.
	vv := buffer.NewViewFromBytes(pkt).ToVectorisedView()
	packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{Data: vv})
	//log.Printf("Upstream packet %v", packetBuffer)

	protocol := header.IPv6ProtocolNumber
	if header.IPVersion(pkt) == header.IPv4Version {
		protocol = header.IPv4ProtocolNumber
	}
	e.InjectInbound(protocol, packetBuffer)
	packetBuffer.DecRef()
	return
}

func NewLink(tunWriter io.WriteCloser) *Endpoint {
	macAddress := tcpip.LinkAddress(string(make([]byte, 6)))
	const pktQueueDepth = 1 // Empirically must be at least 1
	endpoint := channel.New(pktQueueDepth, vpnMtu, macAddress)
	endpoint.AddNotify(&notifyWriter{tunWriter: tunWriter, endpoint: endpoint})
	// FIXME: What about RemoveNotify?

	return &Endpoint{endpoint}
}
