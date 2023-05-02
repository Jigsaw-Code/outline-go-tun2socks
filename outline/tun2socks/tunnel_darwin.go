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

package tun2socks

import (
	"errors"
	"io"
	"runtime/debug"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/shadowsocks"
)

// TunWriter is an interface that allows for outputting packets to the TUN (VPN).
type TunWriter interface {
	io.WriteCloser
}

func init() {
	// Apple VPN extensions have a memory limit of 15MB. Conserve memory by increasing garbage
	// collection frequency and returning memory to the OS every minute.
	debug.SetGCPercent(10)
	// TODO: Check if this is still needed in go 1.13, which returns memory to the OS
	// automatically.
	ticker := time.NewTicker(time.Minute * 1)
	go func() {
		for range ticker.C {
			debug.FreeOSMemory()
		}
	}()
}

// ConnectShadowsocksTunnel reads packets from a TUN device and routes it to a Shadowsocks proxy server.
// Returns an OutlineTunnel instance that should be used to input packets to the tunnel.
//
// `tunWriter` is used to output packets to the TUN (VPN).
// `client` is the Shadowsocks client (created by [shadowsocks.NewClient]).
// `isUDPEnabled` indicates whether the tunnel and/or network enable UDP proxying.
//
// Sets an error if the tunnel fails to connect.
func ConnectShadowsocksTunnel(tunWriter TunWriter, client *shadowsocks.Client, isUDPEnabled bool) (OutlineTunnel, error) {
	if tunWriter == nil {
		return nil, errors.New("must provide a TunWriter")
	} else if client == nil {
		return nil, errors.New("must provide a client")
	}
	return newTunnel(client, client, isUDPEnabled, tunWriter)
}

// ConnectTunnel reads packets from a TUN device represented by `tunDev` and
// routes them to a remote proxy server represented by `configJSON`. This
// function will also do connectivity tests before starting, so the caller is
// not required to do any UDP tests.
//
// If the function succeeds, it will return a nil error and a Tunnel instance.
//
// This function will close `tunDev` after Tunnel disconnects.
func ConnectTunnel(configJSON string, tunDev TunWriter) (Tunnel, error) {
	if len(configJSON) == 0 {
		return nil, errors.New("tunnel configuration is required")
	}
	if tunDev == nil {
		return nil, errors.New("must provide a TunWriter")
	}
	return newTunnelFromJSON(configJSON, tunDev)
	// we don't need to copy packets from tunDev to tn, caller will do so
}
