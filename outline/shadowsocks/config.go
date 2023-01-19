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
// Exported via gobind.
//
// Deprecated: Please use MakeClient().
type Config struct {
	Host       string
	Port       int
	Password   string
	CipherName string
	Prefix     []byte
}

// Must match the ShadowsocksSessionConfig interface in Typescript.
type configJSON struct {
	Host     string
	Port     uint16
	Password string
	Method   string
	Prefix   string
}

// Client provides a transparent container for [client.Client] that
// is exportable (as an opaque object) via gobind.
type Client struct {
	client.Client
}

// The prefix is an 8-bit-clean byte sequence, stored in the codepoint
// values of a unicode string, which arrives here encoded in UTF-8.
func extractPrefixBytes(prefixUtf8 string) ([]byte, error) {
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
//
// Deprecated: Please use MakeClient().
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

// newConfig converts a UTF-8 JSON string into a Config struct.
// A string is used to ensure that configs can be passed through gobind.
func newConfig(s string) (*Config, error) {
	var parsed configJSON
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Host) == 0 || parsed.Port == 0 || len(parsed.Method) == 0 || len(parsed.Password) == 0 {
		return nil, errors.New("missing mandatory field")
	}
	config := Config{
		Host:       parsed.Host,
		Port:       int(parsed.Port),
		CipherName: parsed.Method,
		Password:   parsed.Password,
	}
	prefixBytes, err := extractPrefixBytes(parsed.Prefix)
	if err != nil {
		return nil, fmt.Errorf("prefix parsing failed: %w", err)
	}
	config.Prefix = prefixBytes
	return &config, nil
}

// MakeClient provides a gobind-compatible wrapper for [client.NewClient].
// [configStr] contains a JSON configuration object.
func MakeClient(configStr string) (*Client, error) {
	config, err := newConfig(configStr)
	if err != nil {
		return nil, err
	}
	return NewClient(config)
}
