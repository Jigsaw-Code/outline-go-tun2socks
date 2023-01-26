package shadowsocks

import (
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/proxy"
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
	return proxy.CheckConnectivity(&proxy.Client{Client: client})
}

// CheckServerReachable determines whether the server at `host:port` is reachable over TCP.
// Returns an error if the server is unreachable.
func CheckServerReachable(host string, port int) error {
	return proxy.CheckServerReachable(host, port)
}
