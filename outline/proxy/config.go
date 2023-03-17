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

// This package provides a highly abstracted client interface for use by
// the platform code via `gobind`.  The exposed functions allow the platform
// code to convert an opaque proxy configuration string into an opaque client
// object and check that the client object is functioning (i.e. able to connect).
// This package currently only offers basic Shadowsocks support, but over time it
// may grow to support other protocols without altering the API as seen by the
// platform code.
package proxy

import (
	"encoding/json"
	"errors"
	"fmt"

	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
	"github.com/eycorsican/go-tun2socks/common/log"
)

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
	shadowsocks.Client
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

// newConfig parses a UTF-8 JSON string into a struct.
// A string is used to ensure that configs can be passed through gobind.
func newConfig(s string) (*configJSON, error) {
	var parsed configJSON
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Host) == 0 || parsed.Port == 0 || len(parsed.Method) == 0 || len(parsed.Password) == 0 {
		return nil, errors.New("missing mandatory field")
	}
	return &parsed, nil
}

// NewClient provides a gobind-compatible wrapper for [client.NewClient].
// [configStr] contains a JSON configuration object.
func NewClient(configStr string) (*Client, error) {
	config, err := newConfig(configStr)
	if err != nil {
		return nil, err
	}
	c, err := shadowsocks.NewClient(config.Host, int(config.Port), config.Password, config.Method)
	if err != nil {
		return nil, err
	}
	if len(config.Prefix) > 0 {
		log.Debugf("Using salt prefix: %s", string(config.Prefix))
		prefixBytes, err := extractPrefixBytes(config.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid prefix: %v", err)
		}
		c.SetTCPSaltGenerator(shadowsocks.NewPrefixSaltGenerator(prefixBytes))
	}

	return &Client{c}, nil
}
