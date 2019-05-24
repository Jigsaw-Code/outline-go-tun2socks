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

// Derived from go-tun2socks's "direct" handler under the Apache 2.0 license.

package intra

import (
	"io"
	"net"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	fakedns net.Addr
	truedns net.Addr
}

// NewTCPHandler returns a TCP forwarder with Intra-style behavior.
// Currently this class only redirects DNS traffic to a
// specified server.  (This should be rare for TCP.)
// All other traffic is forwarded unmodified.
func NewTCPHandler(fakedns, truedns net.Addr) core.TCPConnHandler {
	return &tcpHandler{fakedns: fakedns, truedns: truedns}
}

func (h *tcpHandler) handleUpload(local net.Conn, remote *net.TCPConn) {
	// TODO: Handle half-closed sockets more correctly.
	defer func() {
		local.Close()
		remote.CloseWrite()
	}()
	io.Copy(remote, local)
}

func (h *tcpHandler) handleDownload(local net.Conn, remote *net.TCPConn) {
	defer func() {
		local.Close()
		remote.CloseRead()
	}()
	io.Copy(local, remote)
}

// TODO: Request upstream to make `conn` a `core.TCPConn` so we can have finer-
// grained mimicry.
func (h *tcpHandler) Handle(conn net.Conn, target net.Addr) error {
	// DNS override
	// TODO: Consider whether this equality check is acceptable here.
	// (e.g. domain names vs IPs, different serialization of IPv6)
	if target == h.fakedns {
		target = h.truedns
	}
	tcpaddr, err := net.ResolveTCPAddr(target.Network(), target.String())
	if err != nil {
		return err
	}
	c, err := net.DialTCP(target.Network(), nil, tcpaddr)
	if err != nil {
		return err
	}
	go h.handleUpload(conn, c)
	go h.handleDownload(conn, c)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}
