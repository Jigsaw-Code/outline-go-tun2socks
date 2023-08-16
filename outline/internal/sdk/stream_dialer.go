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

package sdk

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Jigsaw-Code/outline-sdk/transport"
	"github.com/Jigsaw-Code/outline-sdk/transport/shadowsocks"
)

// TODO: move to "outline-apps/internal" once we have migrated to monorepo

// NewOutlineStreamDialer creates an outline Shadowsocks StreamDialer from the JSON config.
func NewOutlineStreamDialer(configJSON string) (transport.StreamDialer, error) {
	config, err := parseConfigFromJSON(configJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid Outline configuration: %w", err)
	}

	ssAddress := net.JoinHostPort(config.Hostname, strconv.Itoa(config.Port))
	dialer, err := shadowsocks.NewStreamDialer(&transport.TCPEndpoint{Address: ssAddress}, config.CryptoKey)
	if err != nil {
		return nil, err
	}
	if len(config.Prefix) > 0 {
		dialer.SaltGenerator = shadowsocks.NewPrefixSaltGenerator(config.Prefix)
	}

	return dialer, nil
}
