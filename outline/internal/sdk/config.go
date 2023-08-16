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
	"encoding/json"
	"fmt"

	"github.com/Jigsaw-Code/outline-sdk/transport/shadowsocks"
)

// TODO: move to "outline-apps/internal" once we have migrated to monorepo

// An internal data structure to be used by Outline Shadowsocks transports
type sessionConfig struct {
	Hostname  string
	Port      int
	CryptoKey *shadowsocks.EncryptionKey
	Prefix    []byte
}

// An internal data structure to be used by JSON deserialization.
// Must match the ShadowsocksSessionConfig interface defined in Outline Client.
type configJSON struct {
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Password string `json:"password"`
	Method   string `json:"method"`
	Prefix   string `json:"prefix"`
}

// ParseConfigFromJSON parses a JSON string `in` as a configJSON object.
// The JSON string `in` must match the ShadowsocksSessionConfig interface
// defined in Outline Client.
func parseConfigFromJSON(in string) (config *sessionConfig, err error) {
	var confJson configJSON
	if err = json.Unmarshal([]byte(in), &confJson); err != nil {
		return nil, err
	}

	config = &sessionConfig{
		Hostname: confJson.Host,
		Port:     int(confJson.Port),
	}
	if config.CryptoKey, err = shadowsocks.NewEncryptionKey(confJson.Method, confJson.Password); err != nil {
		return nil, fmt.Errorf("invalid Outline configuration: %w", err)
	}
	if len(confJson.Prefix) > 0 {
		if config.Prefix, err = parseStringPrefix(confJson.Prefix); err != nil {
			return nil, fmt.Errorf("invalid Outline configuration Prefix: %w", err)
		}
	}

	if err = validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid Outline configuration: %w", err)
	}
	return config, nil
}

// validateConfig validates whether a Shadowsocks server configuration is valid
// (it won't do any connectivity tests)
//
// Returns nil if it is valid; or an error message.
func validateConfig(config *sessionConfig) error {
	if len(config.Hostname) == 0 {
		return fmt.Errorf("must provide a host name or IP address")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("port must be within range [1..65535]")
	}
	return nil
}

func parseStringPrefix(utf8Str string) ([]byte, error) {
	runes := []rune(utf8Str)
	rawBytes := make([]byte, len(runes))
	for i, r := range runes {
		if (r & 0xFF) != r {
			return nil, fmt.Errorf("character out of range: %d", r)
		}
		rawBytes[i] = byte(r)
	}
	return rawBytes, nil
}
