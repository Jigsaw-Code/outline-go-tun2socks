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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Jigsaw-Code/outline-ss-server/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

// Config represents a shadowsocks server configuration.
// Exported via gobind.  Must match ShadowsocksSessionConfig interface in Typescript.
//
// TODO: Make this private, so platform code is not exposed to
// Shadowsocks internals.
type Config struct {
	Host     string
	Port     int
	Password string
	Method   string
	Prefix   string
}

// Client provides a transparent container for [client.Client] that
// is exportable (as an opaque object) via gobind.
type Client struct {
	client.Client
}

func extractPrefixBytes(prefixUtf8 string) ([]byte, error) {
	// The prefix is an 8-bit-clean byte sequence, stored in the codepoint
	// values of a unicode string, which arrives here encoded in UTF-8.
	prefixRunes := []rune(prefixUtf8)
	prefixBytes := make([]byte, len(prefixRunes))
	for i, r := range prefixRunes {
		if (r & 0xFF) != r {
			return nil, fmt.Errorf("character out of range: %d", r)
		}
		prefixBytes[i] = byte(r)
	}
	return prefixBytes, nil
}

// NewClient provides a gobind-compatible wrapper for [client.NewClient].
func NewClient(config *Config) (*Client, error) {
	c, err := client.NewClient(config.Host, config.Port, config.Password, config.Method)
	if err != nil {
		return nil, err
	}
	if len(config.Prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(config.Prefix))
		prefixBytes, err := extractPrefixBytes(config.Prefix)
		if err != nil {
			return nil, fmt.Errorf("prefix parsing failed: %w", err)
		}
		c.SetTCPSaltGenerator(client.NewPrefixSaltGenerator(prefixBytes))
	}

	return &Client{c}, nil
}

// NewConfig converts a UTF-8 JSON string into a Config struct.
// A string is used instead of
func NewConfig(s string) (*Config, error) {
	var config Config
	if err := json.Unmarshal([]byte(s), &config); err != nil {
		return nil, err
	}
	if len(config.Host) == 0 || config.Port == 0 || len(config.Method) == 0 || len(config.Password) == 0 {
		return nil, errors.New("missing mandatory field")
	}
	return &config, nil
}
