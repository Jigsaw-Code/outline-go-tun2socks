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
	"fmt"

	"github.com/Jigsaw-Code/outline-go-tun2socks/outline/internal/encoding/utf8"
)

// [Exported] Config represents a shadowsocks server configuration.
type Config struct {
	Host       string
	Port       int
	Password   string
	CipherName string
	Prefix     []byte
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

// [Exported] ParseConfigFromJSON parses a JSON string `in` as a Config object.
// The JSON string `in` must match the ShadowsocksSessionConfig interface
// defined in Outline Client.
func ParseConfigFromJSON(in string) (*Config, error) {
	var tConf configJSON
	if err := json.Unmarshal([]byte(in), &tConf); err != nil {
		return nil, err
	}

	config := Config{
		Host:       tConf.Host,
		Port:       int(tConf.Port),
		Password:   tConf.Password,
		CipherName: tConf.Method,
	}

	if len(tConf.Prefix) > 0 {
		prefixBytes, err := utf8.DecodeCodepointsToBytes(tConf.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid prefix: %w", err)
		}
		config.Prefix = prefixBytes
	}
	return &config, nil
}
