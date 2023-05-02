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

//go:build (linux || windows) && !android

package tun2socks

import (
	"errors"
	"fmt"
	"io"
)

const mtu = 1500

// ConnectTunnel reads packets from a TUN device represented by `tunDev` and
// routes them to a remote proxy server represented by `configJSON`. This
// function will also do connectivity tests before starting, so the caller is
// not required to do any UDP tests.
//
// If the function succeeds, it will return a nil error and a Tunnel instance.
//
// This function will close `tunDev` after Tunnel disconnects.
func ConnectTunnel(configJSON string, tunDev io.ReadWriteCloser) (Tunnel, error) {
	if len(configJSON) == 0 {
		return nil, errors.New("tunnel configuration is required")
	}
	if tunDev == nil {
		return nil, errors.New("tun device is required")
	}
	tn, err := newTunnelFromJSON(configJSON, tunDev)
	if err != nil {
		return nil, fmt.Errorf("unable to create tunnel from config: %w", err)
	}

	go io.CopyBuffer(tn, tunDev, make([]byte, mtu))

	return tn, nil
}
