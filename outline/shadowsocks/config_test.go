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

func Test_extractPrefixBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "basic",
			input: "abc 123",
			want:  []byte("abc 123"),
		}, {
			name:  "empty",
			input: "",
			want:  []byte{},
		}, {
			name:  "extended",
			input: string([]rune{0, 1, 2, 126, 127, 128, 129, 254, 255}),
			want:  []byte{0, 1, 2, 126, 127, 128, 129, 254, 255},
		}, {
			name:    "out of range 256",
			input:   string([]rune{256}),
			wantErr: true,
		}, {
			name:    "out of range 257",
			input:   string([]rune{257}),
			wantErr: true,
		}, {
			name:    "out of range 65537",
			input:   string([]rune{65537}),
			wantErr: true,
		}, {
			name:    "invalid UTF-8",
			input:   "\xc3\x28",
			wantErr: true,
		}, {
			name:    "invalid Unicode",
			input:   "\xf8\xa1\xa1\xa1\xa1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPrefixBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPrefixBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("extractPrefixBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
