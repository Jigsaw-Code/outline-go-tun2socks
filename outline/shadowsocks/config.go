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

// Deprecated: Please use
// [github.com/Jigsaw-Code/outline-go-tun2socks/outline/client] instead.
package shadowsocks

import (
	"github.com/Jigsaw-Code/outline-ss-server/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

// Config represents a shadowsocks server configuration.
// Exported via gobind.
type Config struct {
	Host       string
	Port       int
	Password   string
	CipherName string
	Prefix     []byte
}

// Client provides a transparent container for [client.Client] that
// is exportable (as an opaque object) via gobind.
type Client struct {
	client.Client
}

// NewClient provides a gobind-compatible wrapper for [client.NewClient].
func NewClient(config *Config) (*Client, error) {
	c, err := client.NewClient(config.Host, config.Port, config.Password, config.CipherName)
	if err != nil {
		return nil, err
	}
	if len(config.Prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(config.Prefix))
		c.SetTCPSaltGenerator(client.NewPrefixSaltGenerator(config.Prefix))
	}

	return &Client{c}, nil
}
