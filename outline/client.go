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

package outline

import (
	"fmt"

	internal "github.com/Jigsaw-Code/outline-go-tun2socks/outline/internal/shadowsocks"
	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/internal/utf8"
	"github.com/Jigsaw-Code/outline-internal-sdk/transport"
)

// Client provides a transparent container for [transport.StreamDialer] and [transport.PacketListener]
// that is exportable (as an opaque object) via gobind.
// It's used by the connectivity test and the tun2socks handlers.
type Client interface {
	transport.StreamDialer
	transport.PacketListener
}

// NewClientFromJSON creates a new Shadowsocks client from a JSON
// formatted configuration.
func NewClientFromJSON(configJSON string) (Client, error) {
	config, err := parseConfigFromJSON(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Shadowsocks configuration JSON: %w", err)
	}

	var prefixBytes []byte = nil
	if len(config.Prefix) > 0 {
		if p, err := utf8.DecodeUTF8CodepointsToRawBytes(config.Prefix); err != nil {
			return nil, fmt.Errorf("failed to parse prefix string: %w", err)
		} else {
			prefixBytes = p
		}
	}

	c, err := internal.NewShadowsocksClient(config.Host, int(config.Port), config.Method, config.Password, prefixBytes)
	if err != nil {
		// A <nil> struct is not a <nil> interface
		return nil, err
	}

	return c, err
}
