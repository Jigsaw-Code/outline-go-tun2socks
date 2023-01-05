package shadowsocks

import (
	"errors"
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
func CheckConnectivity(client *Client) (int, error) {
	// Start asynchronous UDP support check.
	udpChan := make(chan error)
	go func() {
		udpChan <- oss.CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("1.1.1.1:53", "udp"))
	}()
	// Check whether the proxy is reachable and that the client is able to authenticate to the proxy
	tcpErr := oss.CheckTCPConnectivityWithHTTP(client, "http://example.com")
	if tcpErr == nil {
		udpErr := <-udpChan
		if udpErr == nil {
			return NoError, nil
		}
		return UDPConnectivity, nil
	}
	var authErr *oss.AuthenticationError
	var reachabilityErr *oss.ReachabilityError
	if errors.As(tcpErr, &authErr) {
		return AuthenticationFailure, nil
	} else if errors.As(tcpErr, &reachabilityErr) {
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
