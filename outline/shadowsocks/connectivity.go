package shadowsocks

import (
	"net"
	"strconv"
	"time"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

// Outline error codes. Must be kept in sync with definitions in outline-client/cordova-plugin-outline/outlinePlugin.js
const (
	NoError                     = 0
	Unexpected                  = 1
	NoVPNPermissions            = 2
	AuthenticationFailure       = 3
	UDPConnectivity             = 4
	Unreachable                 = 5
	VpnStartFailure             = 6
	IllegalConfiguration        = 7
	ShadowsocksStartFailure     = 8
	ConfigureSystemProxyFailure = 9
	NoAdminPermissions          = 10
	UnsupportedRoutingTable     = 11
	SystemMisconfigured         = 12
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
		return Unexpected, err
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
		return NoError, nil
	}
	tcpErr := <-tcpChan
	if tcpErr == nil {
		// The TCP connectivity checks succeeded, which means UDP is not supported.
		return UDPConnectivity, nil
	}
	_, isReachabilityError := tcpErr.(*oss.ReachabilityError)
	_, isAuthError := tcpErr.(*oss.AuthenticationError)
	if isAuthError {
		return AuthenticationFailure, nil
	} else if isReachabilityError {
		return Unreachable, nil
	}
	// The error is not related to the connectivity checks.
	return Unexpected, tcpErr
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
