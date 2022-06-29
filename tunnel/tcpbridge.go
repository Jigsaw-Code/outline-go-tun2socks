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
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
)

type bridgeConn struct {
	endpoint    tcpip.Endpoint
	q           *waiter.Queue
	closedMu    sync.Mutex
	readClosed  bool
	writeClosed bool
}

func (c *bridgeConn) WriteTo(w io.Writer) (n int64, err error) {
	waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventIn)
	c.q.EventRegister(&waitEntry)
	defer c.q.EventUnregister(&waitEntry)
	for {
		result, tcpErr := c.endpoint.Read(w, tcpip.ReadOptions{})
		if _, ok := tcpErr.(*tcpip.ErrWouldBlock); ok {
			<-notifyCh
			continue
		}
		n += int64(result.Total)
		if _, ok := tcpErr.(*tcpip.ErrClosedForReceive); ok {
			// EOF case.  Success.
			return
		} else if tcpErr != nil {
			err = errors.New(tcpErr.String())
			return
		}
	}
}

func (c *bridgeConn) Write(b []byte) (int, error) {
	waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventOut)
	c.q.EventRegister(&waitEntry)
	defer c.q.EventUnregister(&waitEntry)
	reader := bytes.NewReader(b)
	for {
		_, tcpErr := c.endpoint.Write(reader, tcpip.WriteOptions{Atomic: true})
		_, isErrWouldBlock := tcpErr.(*tcpip.ErrWouldBlock)
		if tcpErr != nil && !isErrWouldBlock {
			return len(b) - reader.Len(), errors.New(tcpErr.String()) // TODO EOF?
		} else if reader.Len() == 0 {
			return len(b), nil
		}
		<-notifyCh
	}
}

func (c *bridgeConn) Close() error {
	c.endpoint.Close()
	return nil
}

func (c *bridgeConn) CloseRead() error {
	err := c.endpoint.Shutdown(tcpip.ShutdownRead)
	c.closedMu.Lock()
	c.readClosed = true
	if c.writeClosed {
		c.endpoint.Close()
	}
	c.closedMu.Unlock()
	if err != nil {
		return errors.New(err.String())
	}
	return nil
}

func (c *bridgeConn) CloseWrite() error {
	err := c.endpoint.Shutdown(tcpip.ShutdownWrite)
	c.closedMu.Lock()
	c.writeClosed = true
	if c.readClosed {
		c.endpoint.Close()
	}
	c.closedMu.Unlock()
	if err != nil {
		return errors.New(err.String())
	}
	return nil
}

func tcpbridge(dialer core.TCPDialer, r *tcp.ForwarderRequest) {
	id := r.ID()
	dstAddr := net.TCPAddr{
		IP:   []byte(id.LocalAddress),
		Port: int(id.LocalPort),
	}
	log.Printf("Dialing %s", dstAddr.String())
	toDest, err := dialer.DialTCP(&dstAddr)
	if err != nil {
		log.Printf("Proxying failed: %v", err)
		// TODO: Don't emit a RST if the error was a timeout.
		r.Complete(true) // RST
		return
	}

	var q waiter.Queue
	endpoint, stackerr := r.CreateEndpoint(&q) // SYNACK
	if stackerr != nil {
		log.Printf("Endpoint creation failed for request %+v (%s): %v", r, dstAddr.String(), stackerr)
		r.Complete(true) // RST
		return
	}

	toClient := bridgeConn{endpoint: endpoint, q: &q}
	r.Complete(false)
	go toClient.WriteTo(toDest)
	go io.Copy(&toClient, toDest)
}

func tcphandler(dialer core.TCPDialer) func(*tcp.ForwarderRequest) {
	// The returned function will be run in a fresh goroutine for each incoming socket.
	return func(r *tcp.ForwarderRequest) {
		tcpbridge(dialer, r)
	}
}

func MakeTCPBridge(s *stack.Stack, dialer core.TCPDialer) *tcp.Forwarder {
	return tcp.NewForwarder(s, 0, 10, tcphandler(dialer))
}
