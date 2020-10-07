package shadowsocks

import (
	"errors"
	"net"
	"net/http"
	"time"

	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

// TODO: make these values configurable by exposing a struct with the connectivity methods.
const (
	tcpTimeoutMs        = udpTimeoutMs * udpMaxRetryAttempts
	udpTimeoutMs        = 1000
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

// CheckUDPConnectivityWithDNS determines whether the Shadowsocks proxy represented by `client` and
// the network support UDP traffic by issuing a DNS query though a resolver at `resolverAddr`.
// Returns nil on success or an error on failure.
func CheckUDPConnectivityWithDNS(client shadowsocks.Client, resolverAddr net.Addr) error {
	conn, err := client.ListenUDP(nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, bufferLength)
	for attempt := 0; attempt < udpMaxRetryAttempts; attempt++ {
		conn.SetDeadline(time.Now().Add(time.Millisecond * udpTimeoutMs))
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

// CheckTCPConnectivityWithHTTP determines whether the proxy is reachable over TCP and validates the
// client's authentication credentials by performing an HTTP HEAD request to `targetURL`, which must
// be of the form: http://[host](:[port])(/[path]). Returns nil on success, error if `targetURL` is
// invalid, AuthenticationError or ReachabilityError on connectivity failure.
func CheckTCPConnectivityWithHTTP(client shadowsocks.Client, targetURL string) error {
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		return err
	}
	targetAddr := req.Host
	if !hasPort(targetAddr) {
		targetAddr = net.JoinHostPort(targetAddr, "80")
	}
	conn, err := client.DialTCP(nil, targetAddr)
	if err != nil {
		return &ReachabilityError{err}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Millisecond * tcpTimeoutMs))
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
