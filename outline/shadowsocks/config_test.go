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
	"bytes"
	"testing"
)

func Test_ParseConfigFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Config
		wantErr bool
	}{
		{
			name:  "normal config",
			input: `{"host":"192.0.2.1","port":12345,"method":"some-cipher","password":"abcd1234"}`,
			want: &Config{
				Host:       "192.0.2.1",
				Port:       12345,
				CipherName: "some-cipher",
				Password:   "abcd1234",
				Prefix:     nil,
			},
		},
		{
			name:  "normal config with prefix",
			input: `{"host":"192.0.2.1","port":12345,"method":"some-cipher","password":"abcd1234","prefix":"abc 123"}`,
			want: &Config{
				Host:       "192.0.2.1",
				Port:       12345,
				CipherName: "some-cipher",
				Password:   "abcd1234",
				Prefix:     []byte{97, 98, 99, 32, 49, 50, 51},
			},
		},
		{
			name:  "normal config with extra fields",
			input: `{"extra_field":"ignored","host":"192.0.2.1","port":12345,"method":"some-cipher","password":"abcd1234"}`,
			want: &Config{
				Host:       "192.0.2.1",
				Port:       12345,
				CipherName: "some-cipher",
				Password:   "abcd1234",
				Prefix:     nil,
			},
		},
		{
			name:    "missing host",
			input:   `{"port":12345,"method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "missing port",
			input:   `{"host":"192.0.2.1","method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "missing method",
			input:   `{"host":"192.0.2.1","port":12345,"password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "missing password",
			input:   `{"host":"192.0.2.1","port":12345,"method":"some-cipher"}`,
			wantErr: true,
		},
		{
			name:    "empty host",
			input:   `{"host":"","port":12345,"method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "zero port",
			input:   `{"host":"192.0.2.1","port":0,"method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "empty method",
			input:   `{"host":"192.0.2.1","port":12345,"method":"","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "empty password",
			input:   `{"host":"192.0.2.1","port":12345,"method":"some-cipher","password":""}`,
			wantErr: true,
		},
		{
			name:    "port -1",
			input:   `{"host":"192.0.2.1","port":-1,"method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
		{
			name:    "port 65536",
			input:   `{"host":"192.0.2.1","port":65536,"method":"some-cipher","password":"abcd1234"}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfigFromJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("newConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.want.Host ||
				got.Port != tt.want.Port ||
				got.CipherName != tt.want.CipherName ||
				got.Password != tt.want.Password ||
				!bytes.Equal(got.Prefix, tt.want.Prefix) {
				t.Errorf("newConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
