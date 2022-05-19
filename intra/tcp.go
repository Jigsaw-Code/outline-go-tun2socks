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
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"github.com/Jigsaw-Code/outline-go-tun2socks/dns"

	"github.com/Jigsaw-Code/outline-go-tun2socks/intra/split"

	split_routing "github.com/Jigsaw-Code/outline-go-tun2socks/split"
)

// TCPDialer is a core TCP handler that also supports DOH and splitting control.
type TCPDialer interface {
	core.TCPDialer
	SetDNS(dns.Transport)
	SetAlwaysSplitHTTPS(bool)
	EnableSNIReporter(file io.ReadWriter, suffix, country string) error
}

type tcpDialer struct {
	core.TCPDialer
	dialer      *net.Dialer
	httpsDialer changeDialer
	dnsDialer   dns.TCPDialer

	listener    TCPListener
	sniReporter tcpSNIReporter
}

// TCPSocketSummary provides information about each TCP socket, reported when it is closed.
type TCPSocketSummary struct {
	DownloadBytes int64 // Total bytes downloaded.
	UploadBytes   int64 // Total bytes uploaded.
	Duration      int32 // Duration in seconds.
	ServerPort    int16 // The server port.  All values except 80, 443, and 0 are set to -1.
	Synack        int32 // TCP handshake latency (ms)
	// Retry is non-nil if retry was possible.  Retry.Split is non-zero if a retry occurred.
	Retry *split.RetryStats
}

// TCPListener is notified when a socket closes.
type TCPListener interface {
	OnTCPSocketClosed(*TCPSocketSummary)
}

type measuringDialer struct {
	engine      core.TCPDialer
	listener    TCPListener
	sniReporter *tcpSNIReporter
}

func (d measuringDialer) DialTCP(dst *net.TCPAddr) (core.DoubleStream, error) {
	start := time.Now()
	inner, err := d.engine.DialTCP(dst)
	if err != nil {
		return nil, err
	}
	synack := time.Since(start)
	return makeMeasuredConn(inner, d.listener, d.sniReporter, synack, dst), nil
}

type measuredConn struct {
	inner         core.DoubleStream
	listener      TCPListener
	sniReporter   *tcpSNIReporter
	synack        time.Duration
	serverPort    int16
	start         time.Time
	uploadBytes   int64
	downloadBytes int64
	readClosed    bool
	writeClosed   bool
}

func makeMeasuredConn(inner core.DoubleStream, listener TCPListener, sniReporter *tcpSNIReporter, synack time.Duration, server net.Addr) core.DoubleStream {
	return &measuredConn{
		inner:       inner,
		listener:    listener,
		sniReporter: sniReporter,
		synack:      synack,
		serverPort:  filteredPort(server),
		start:       time.Now(), // FIXME Confirm that this is the same start time (vs. before synack)
	}
}

func (c *measuredConn) Read(b []byte) (int, error) {
	n, err := c.inner.Read(b)
	c.uploadBytes += int64(n)
	return n, err
}

func (c *measuredConn) WriteTo(w io.Writer) (int64, error) {
	n, err := io.Copy(w, c.inner)
	c.uploadBytes += n
	return n, err
}

func (c *measuredConn) Write(b []byte) (int, error) {
	n, err := c.inner.Write(b)
	c.downloadBytes += int64(n)
	return n, err
}

func (c *measuredConn) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.Copy(c.inner, r)
	c.downloadBytes += n
	return n, err
}

func (c *measuredConn) CloseRead() error {
	if !c.readClosed {
		c.readClosed = true
		c.finalize()
	}
	return c.inner.CloseRead()
}

func (c *measuredConn) CloseWrite() error {
	if !c.writeClosed {
		c.writeClosed = true
		c.finalize()
	}
	return c.inner.CloseWrite()
}

func (c *measuredConn) finalize() {
	// Wait until both ends are closed
	if !c.readClosed || !c.writeClosed {
		return
	}
	summary := TCPSocketSummary{
		DownloadBytes: c.downloadBytes,
		UploadBytes:   c.uploadBytes,
		Duration:      int32(time.Since(c.start).Seconds()),
		ServerPort:    c.serverPort,
		Synack:        int32(c.synack.Milliseconds()),
	}
	if cast, ok := c.inner.(split.HasRetryStats); ok {
		retry := cast.GetRetryStats()
		summary.Retry = &retry
		c.sniReporter.Report(summary)
	}
	c.listener.OnTCPSocketClosed(&summary)
}

type changeDialer struct {
	mu    sync.RWMutex
	inner core.TCPDialer
}

func (d *changeDialer) DialTCP(dst *net.TCPAddr) (core.DoubleStream, error) {
	d.mu.RLock()
	dialer := d.inner
	d.mu.RUnlock()
	return dialer.DialTCP(dst)
}

func (d *changeDialer) Change(inner core.TCPDialer) {
	d.mu.Lock()
	d.inner = inner
	d.mu.Unlock()
}

// NewTCPDialer returns a TCP dialer with Intra-style behavior.
// Connections to `fakedns` are redirected to DOH.
// All other traffic is forwarded using `dialer`.
// `listener` is provided with a summary of each socket when it is closed.
func NewTCPDialer(fakedns net.TCPAddr, dialer *net.Dialer, listener TCPListener) TCPDialer {
	ret := tcpDialer{
		dialer: dialer,
		httpsDialer: changeDialer{
			inner: split.MakeRetryDialer(dialer),
		},
		dnsDialer: dns.NewTCPDialer(),
		listener:  listener,
	}
	ret.TCPDialer = split_routing.NewTCPDialer(
		split_routing.MakeOneIPRule(fakedns.IP),
		ret.dnsDialer,
		&measuringDialer{
			engine: split_routing.NewTCPDialer(
				split_routing.MakeOnePortRule(443),
				&ret.httpsDialer,
				core.MakeTCPDialer(dialer),
			),
			listener:    listener,
			sniReporter: &ret.sniReporter,
		})
	return &ret
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

func (h *tcpDialer) SetDNS(dns dns.Transport) {
	h.dnsDialer.SetDNS(dns)
	h.sniReporter.SetDNS(dns)
}

func (h *tcpDialer) SetAlwaysSplitHTTPS(s bool) {
	if s {
		h.httpsDialer.inner = split.MakeAlwaysSplitDialer(h.dialer)
	} else {
		h.httpsDialer.inner = split.MakeRetryDialer(h.dialer)
	}
}

func (h *tcpDialer) EnableSNIReporter(file io.ReadWriter, suffix, country string) error {
	return h.sniReporter.Configure(file, suffix, country)
}
