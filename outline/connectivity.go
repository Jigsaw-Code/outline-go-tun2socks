// Copyright 2023 The Outline Authors
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

package outline

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
)

// TODO: make these values configurable by exposing a struct with the connectivity methods.
const (
	tcpTimeout          = 10 * time.Second
	udpTimeout          = 1 * time.Second
	udpMaxRetryAttempts = 5
	bufferLength        = 512
)

// AuthenticationError is used to signal failed authentication to the Shadowsocks proxy.
type AuthenticationError struct {
	error
}

// ReachabilityError is used to signal an unreachable proxy.
type ReachabilityError struct {
	error
}

// checkUDPConnectivityWithDNS determines whether the Shadowsocks proxy represented by `client` and
// the network support UDP traffic by issuing a DNS query though a resolver at `resolverAddr`.
// Returns nil on success or an error on failure.
func CheckUDPConnectivityWithDNS(client transport.PacketListener, resolverAddr net.Addr) error {
	conn, err := client.ListenPacket(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, bufferLength)
	for attempt := 0; attempt < udpMaxRetryAttempts; attempt++ {
		conn.SetDeadline(time.Now().Add(udpTimeout))
		_, err := conn.WriteTo(getDNSRequest(), resolverAddr)
		if err != nil {
			continue
		}
		n, addr, err := conn.ReadFrom(buf)
		if n == 0 && err != nil {
			continue
		}
		if addr.String() != resolverAddr.String() {
			continue // Ensure we got a response from the resolver.
		}
		return nil
	}
	return errors.New("UDP connectivity check timed out")
}

// checkTCPConnectivityWithHTTP determines whether the proxy is reachable over TCP and validates the
// client's authentication credentials by performing an HTTP HEAD request to `targetURL`, which must
// be of the form: http://[host](:[port])(/[path]). Returns nil on success, error if `targetURL` is
// invalid, AuthenticationError or ReachabilityError on connectivity failure.
func CheckTCPConnectivityWithHTTP(dialer transport.StreamDialer, targetURL string) error {
	deadline := time.Now().Add(tcpTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		return err
	}
	targetAddr := req.Host
	if !hasPort(targetAddr) {
		targetAddr = net.JoinHostPort(targetAddr, "80")
	}
	conn, err := dialer.Dial(ctx, targetAddr)
	if err != nil {
		return &ReachabilityError{err}
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	err = req.Write(conn)
	if err != nil {
		return &AuthenticationError{err}
	}
	n, err := conn.Read(make([]byte, bufferLength))
	if n == 0 && err != nil {
		return &AuthenticationError{err}
	}
	return nil
}

func getDNSRequest() []byte {
	return []byte{
		0, 0, // [0-1]   query ID
		1, 0, // [2-3]   flags; byte[2] = 1 for recursion desired (RD).
		0, 1, // [4-5]   QDCOUNT (number of queries)
		0, 0, // [6-7]   ANCOUNT (number of answers)
		0, 0, // [8-9]   NSCOUNT (number of name server records)
		0, 0, // [10-11] ARCOUNT (number of additional records)
		3, 'c', 'o', 'm',
		0,    // null terminator of FQDN (root TLD)
		0, 1, // QTYPE, set to A
		0, 1, // QCLASS, set to 1 = IN (Internet)
	}
}

func hasPort(hostPort string) bool {
	_, _, err := net.SplitHostPort(hostPort)
	return err == nil
}
