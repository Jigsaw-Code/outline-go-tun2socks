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
	"time"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	fakedns          net.TCPAddr
	truedns          net.TCPAddr
	alwaysSplitHTTPS bool
	listener         TCPListener
}

// Usage summary for each TCP socket, reported when it is closed.
type TCPSocketSummary struct {
	DownloadBytes int64       // Total bytes downloaded.
	UploadBytes   int64       // Total bytes uploaded.
	Duration      int32       // Duration in seconds.
	ServerPort    int16       // The server port.  All values except 80, 443, and 0 are set to -1.
	Synack        int32       // TCP handshake latency (ms)
	Retry         *RetryStats // Non-nil if a retry occurred.
}

type TCPListener interface {
	OnTCPSocketClosed(*TCPSocketSummary)
}

type DuplexConn interface {
	io.ReadWriter
	io.ReaderFrom
	CloseWrite() error
	CloseRead() error
}

// NewTCPHandler returns a TCP forwarder with Intra-style behavior.
// Currently this class only redirects DNS traffic to a
// specified server.  (This should be rare for TCP.)
// All other traffic is forwarded unmodified.
func NewTCPHandler(fakedns, truedns net.TCPAddr, alwaysSplitHTTPS bool, listener TCPListener) core.TCPConnHandler {
	return &tcpHandler{
		fakedns:          fakedns,
		truedns:          truedns,
		alwaysSplitHTTPS: alwaysSplitHTTPS,
		listener:         listener,
	}
}

// TODO: Propagate TCP RST using local.Abort(), on appropriate errors.
func (h *tcpHandler) handleUpload(local core.TCPConn, remote DuplexConn, upload chan int64) {
	bytes, _ := remote.ReadFrom(local)
	local.CloseRead()
	remote.CloseWrite()
	upload <- bytes
}

func (h *tcpHandler) handleDownload(local core.TCPConn, remote DuplexConn) (bytes int64, err error) {
	bytes, err = io.Copy(local, remote)
	local.CloseWrite()
	remote.CloseRead()
	return
}

func (h *tcpHandler) forward(local net.Conn, remote DuplexConn, summary *TCPSocketSummary) {
	localtcp := local.(core.TCPConn)
	upload := make(chan int64)
	start := time.Now()
	go h.handleUpload(localtcp, remote, upload)
	download, _ := h.handleDownload(localtcp, remote)
	summary.DownloadBytes = download
	summary.UploadBytes = <-upload
	summary.Duration = int32(time.Since(start).Seconds())
	h.listener.OnTCPSocketClosed(summary)
}

func filteredPort(addr net.Addr) int16 {
	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return -1
	}
	if port == "80" {
		return 80
	}
	if port == "443" {
		return 443
	}
	if port == "0" {
		return 0
	}
	return -1
}

// TODO: Request upstream to make `conn` a `core.TCPConn` so we can avoid a type assertion.
func (h *tcpHandler) Handle(conn net.Conn, target *net.TCPAddr) error {
	// DNS override
	if target.IP.Equal(h.fakedns.IP) && target.Port == h.fakedns.Port {
		target = &h.truedns
	}
	var summary TCPSocketSummary
	summary.ServerPort = filteredPort(target)
	start := time.Now()
	var c DuplexConn
	var err error
	if summary.ServerPort == 443 {
		if h.alwaysSplitHTTPS {
			c, err = DialWithSplit(target.Network(), target)
		} else {
			c, err = DialWithSplitRetry(target.Network(), target, &summary)
		}
	} else {
		c, err = net.DialTCP(target.Network(), nil, target)
	}
	if err != nil {
		return err
	}
	summary.Synack = int32(time.Since(start).Seconds() * 1000)
	go h.forward(conn, c, &summary)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}
