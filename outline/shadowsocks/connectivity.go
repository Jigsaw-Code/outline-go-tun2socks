package shadowsocks

import (
	"net"
	"strconv"
	"time"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
)

// Outline error codes. Must be kept in sync with definitions in outline-client/cordova-plugin-outline/outlinePlugin.js
const (
	noError                     = 0
	unexpected                  = 1
	noVPNPermissions            = 2
	authenticationFailure       = 3
	udpConnectivity             = 4
	unreachable                 = 5
	vpnStartFailure             = 6
	ilegalConfiguration         = 7
	shadowsocksStartFailure     = 8
	configureSystemProxyFailure = 9
	noAdminPermissions          = 10
	unsupportedRoutingTable     = 11
	systemMisconfigured         = 12
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
	tcpChan := make(chan error)
	// Check whether the proxy is reachable and that the client is able to authenticate to the proxy
	go func() {
		tcpChan <- oss.CheckTCPConnectivityWithHTTP(client, "http://example.com")
	}()
	// Check whether UDP is supported
	udpErr := oss.CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("1.1.1.1:53", "udp"))
	if udpErr == nil {
		// The UDP connectvity check is a superset of the TCP checks. If the other tests fail,
		// assume it's due to intermittent network conditions and declare success anyway.
		return noError, nil
	}
	tcpErr := <-tcpChan
	if tcpErr == nil {
		// The TCP connectivity checks succeeded, which means UDP is not supported.
		return udpConnectivity, nil
	}
	_, isReachabilityError := tcpErr.(*oss.ReachabilityError)
	_, isAuthError := tcpErr.(*oss.AuthenticationError)
	if isAuthError {
		return authenticationFailure, nil
	} else if isReachabilityError {
		return unreachable, nil
	}
	// The error is not related to the connectivity checks.
	return unexpected, tcpErr
}

// CheckServerReachable determines whether the server at `host:port` is reachable over TCP.
// Returns an error if the server is unreachable.
func CheckServerReachable(host string, port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), reachabilityTimeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
