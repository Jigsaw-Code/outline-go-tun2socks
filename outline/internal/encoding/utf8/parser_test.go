package utf8

import (
	"bytes"
	"testing"
)

func Test_DecodeCodepointsToBytes(t *testing.T) {
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
			name:  "edge cases (explicit)",
			input: "\x00\x01\x02 \x7e\x7f \xc2\x80\xc2\x81 \xc3\xbd\xc3\xbf",
			want:  []byte("\x00\x01\x02 \x7e\x7f \x80\x81 \xfd\xff"),
		}, {
			name:  "edge cases (roundtrip)",
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
			got, err := DecodeCodepointsToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeCodepointsToBytes() returns error %v, want error %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("DecodeCodepointsToBytes() returns %v, want %v", got, tt.want)
			}
		})
	}
}
