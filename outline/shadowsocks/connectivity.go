package shadowsocks

import (
	"net"
	"strconv"
	"time"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
)

// Outline error codes. Must be kept in sync with definitions in outline-client/corodva-plugin-outline/outlinePlugin.js
const (
	noError int = iota
	unexpected
	noVPNPermissions
	authentication
	udpConnectivity
	unreachable
	vpnStartFailure
	ilegalConfiguration
	shadowsocksStartFailure
	configureSystemProxyFailure
	noAdminPermissions
	unsupportedRoutingTable
	systemMisconfigured
)

const reachabilityTimeout = 10 * time.Second

// CheckConnectivity determines whether the Shadowsocks proxy can relay TCP and UDP traffic under
// the current network. Parallelizes the execution of TCP and UDP checks, selects the appropriate
// error code to return accounting for transient network failures.
// Returns an error if an unexpected error ocurrs.
func CheckConnectivity(host string, port int, password, cipher string) (int, error) {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		// TODO: Inspect error for invalid cipher error or proxy host resolution failure.
		return unexpected, err
	}
	tcpChan := make(chan error, 1)
	udpChan := make(chan error, 1)
	// Check whether the proxy is reachable and that the client is able to authenticate to the proxy
	go func() {
		tcpChan <- oss.CheckTCPConnectivityWithHTTP(client, "http://example.com")
	}()
	// Check whether UDP is supported
	go func() {
		udpChan <- oss.CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("1.1.1.1:53", "udp"))
	}()
	tcpErr := <-tcpChan
	udpErr := <-udpChan

	if udpErr == nil {
		// The UDP connectvity check is a superset of the TCP checks. If the other tests fail,
		// assume it's due to intermittent network conditions and declare success anyway.
		return noError, nil
	} else if tcpErr == nil {
		// The TCP connectivity checks succeeded, which means UDP is not supported.
		return udpConnectivity, nil
	}
	_, isRachabilityError := tcpErr.(*oss.ReachabilityError)
	_, isAuthError := tcpErr.(*oss.AuthenticationError)
	if !isRachabilityError && !isAuthError {
		// The error is not related to the connectivity checks.
		return unexpected, tcpErr
	} else if !isRachabilityError {
		// Proxy is reachable, which means the authentication check failed.
		return authentication, nil
	}
	// All the checks failed because the proxy is unreachable.
	return unreachable, nil
}

// CheckServerReachable determines whether the server at `host:port` is reachable over TCP.
// Returns an error if the server is unreachable.
func CheckServerReachable(host string, port int, timeoutMs int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), reachabilityTimeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// CheckUDPConnectivity determines whether UDP forwarding is supported by a Shadowsocks proxy.
// Returns an error if the server does not support UDP.
// TODO: remove this once we support a Shadowsocks tunnel and deprecate the SOCKS interface,
// as we will be able to perform the UDP connectivity check directly from Go.
func CheckUDPConnectivity(host string, port int, password, cipher string) error {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return err
	}
	return oss.CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("1.1.1.1:53", "udp"))
}
