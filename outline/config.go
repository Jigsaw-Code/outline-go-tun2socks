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

package outline

import (
	"encoding/json"
)

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
func parseConfigFromJSON(in string) (*configJSON, error) {
	var conf configJSON
	if err := json.Unmarshal([]byte(in), &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
