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
	"log"
	"os"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tun2socks"
)

const vpnMtu = 1500

var tun *os.File
var tunnel AndroidTunnel

// AndroidTunnel embeds the tun2socks.Tunnel interface so it gets exported by gobind.
type AndroidTunnel interface {
	tun2socks.Tunnel
}

// ConnectSocksTunnel reads packets from a TUN device and routes it to a SOCKS server. Returns an
// AndroidTunnel instance and does *not* take ownership of the TUN file descriptor; the
// caller is responsible for closing after AndroidTunnel disconnects.
//
// `fd` is the file descriptor to the VPN TUN device. Must be set to blocking mode.
// `host` is  IP address of the SOCKS proxy server.
// `port` is the port of the SOCKS proxy server.
// `isUDPEnabled` indicates whether the tunnel and/or network enable UDP proxying.
//
// Throws an exception if the TUN file descriptor cannot be opened, or if the tunnel fails to
// connect.
func ConnectSocksTunnel(fd int, host string, port int, isUDPEnabled bool) (AndroidTunnel, error) {
	if port <= 0 || port > 65535 {
		return nil, errors.New("Must provide a valid port number")
	}
	if fd < 0 {
		return nil, errors.New("Must provide a valid TUN file descriptor")
	}
	tun = os.NewFile(uintptr(fd), "")
	if tun == nil {
		return nil, errors.New("Failed to open TUN file descriptor")
	}
	var err error
	tunnel, err = tun2socks.NewTunnel(host, uint16(port), isUDPEnabled, tun)
	if err != nil {
		return nil, err
	}
	go processInputPackets()
	return tunnel, nil
}

func processInputPackets() {
	buffer := make([]byte, vpnMtu)
	for tunnel.IsConnected() {
		len, err := tun.Read(buffer)
		if err != nil {
			log.Printf("Failed to read packet from TUN: %v", err)
			continue
		}
		if len == 0 {
			log.Println("Read EOF from TUN")
			continue
		}
		tunnel.Write(buffer)
	}
}
