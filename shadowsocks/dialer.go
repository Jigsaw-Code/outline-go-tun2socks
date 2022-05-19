// Copyright 2022 The Outline Authors
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

package shadowsocks

import (
	"net"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

type Dialer struct {
	Client shadowsocks.Client
}

func (d Dialer) DialTCP(addr *net.TCPAddr) (core.DoubleStream, error) {
	return d.Client.DialTCP(nil, addr.String())
}

func (d Dialer) ListenUDP(_ core.ReverseUDPConn) (net.PacketConn, error) {
	return d.Client.ListenUDP(nil)
}
