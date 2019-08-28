package shadowsocks

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
)

const (
	tcpTimeoutMs        = udpTimeoutMs * udpMaxRetryAttempts
	udpTimeoutMs        = 1000
	udpMaxRetryAttempts = 5
	bufferLength        = 512
)

// ConnectivityResult holds the result of a connectivity check.
type ConnectivityResult struct {
	// Whehter the proxy is reachable through TCP.
	IsReachable bool
	// Whether the client's authentication credentials are valid.
	IsAuthenticated bool
	// Whether the proxy supports UDP forwarding.
	IsUDPSupported bool
}

// CheckConnectivity determines whether the Shadowsocks proxy associated to `client` can relay TCP
// and UDP traffic under the current network.
func CheckConnectivity(client shadowsocks.Client) *ConnectivityResult {
	tcpChan := make(chan error, 1)
	udpChan := make(chan error, 1)
	// Check whether the proxy is reachable and that the client is able to authenticate to the proxy
	go func() {
		tcpChan <- isTCPSupported(client, "example.com:80")
	}()
	// Check whether UDP is supported
	go func() {
		udpChan <- IsUDPSupported(client, shadowsocks.NewAddr("1.1.1.1:53", "udp"))
	}()

	result := &ConnectivityResult{IsReachable: true, IsAuthenticated: true}
	tcpErr := <-tcpChan
	if tcpErr != nil {
		// Determine whether the proxy was reachable over TCP based on the error type; reachability
		// errors occur when dialing to the proxy, authentication errors occur during reads.
		_, isDialError := tcpErr.(*net.OpError)
		result.IsReachable = !isDialError
		result.IsAuthenticated = false
	}
	result.IsUDPSupported = <-udpChan == nil
	return result
}

// IsUDPSupported determines whether the Shadowsocks proxy represented by `client` and the network
// support UDP traffic by resolving a DNS query though a resolver at `resolverAddr`.
// Returns nil on success or an error on failure.
func IsUDPSupported(client shadowsocks.Client, resolverAddr net.Addr) error {
	conn, err := client.ListenUDP(nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Millisecond * udpTimeoutMs))
	buf := make([]byte, bufferLength)
	for attempt := 0; attempt < udpMaxRetryAttempts; attempt++ {
		_, err := conn.WriteTo(getDNSRequest(), resolverAddr)
		if err != nil {
			continue
		}
		n, _, err := conn.ReadFrom(buf)
		if n == 0 && err != nil {
			continue
		}
		return nil
	}
	return errors.New("UDP not supported")
}

// isTCPSupported determines whether the proxy is reachable over TCP and validates the client's
// authentication credentials by performing an HTTP HEAD request to `targetAddr`.
func isTCPSupported(client shadowsocks.Client, targetAddr string) (err error) {
	conn, err := client.DialTCP(nil, targetAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Millisecond * tcpTimeoutMs))
	targetHost, _, _ := net.SplitHostPort(targetAddr) // Ignore error, DialTCP would have failed
	payload := fmt.Sprintf("HEAD / HTTP/1.1\r\nHost: %v\r\n\r\n", targetHost)
	_, err = conn.Write([]byte(payload))
	if err != nil {
		return
	}
	n, err := conn.Read(make([]byte, bufferLength))
	if n == 0 && err != nil {
		return
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
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,    // null terminator of FQDN (root TLD)
		0, 1, // QTYPE, set to A
		0, 1, // QCLASS, set to 1 = IN (Internet)
	}
}
