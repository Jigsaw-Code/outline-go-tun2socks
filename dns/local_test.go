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

package dns

import (
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

var queryBytes []byte = []byte{
	0xbe, 0xef, // ID
	0x01,       // QR, OPCODE, AA, TC, RD
	0x00,       // RA, Z, RCODE
	0x00, 0x01, // QDCOUNT = 1
	0x00, 0x00, // ANCOUNT = 0
	0x00, 0x00, // NSCOUNT = 0
	0x00, 0x00, // ARCOUNT = 0
	// Question 1
	0x03, 'd', 'n', 's',
	0x06, 'g', 'o', 'o', 'g', 'l', 'e',
	0x00,
	0x00, 0x01, // QTYPE: A query
	0x00, 0x01, // QCLASS: IN
}

func TestLocalQuery(t *testing.T) {
	local := MakeLocalTransport()
	resp, err := local.Query(queryBytes)
	if err != nil {
		t.Error(err)
	}
	var msg dnsmessage.Message
	if err := msg.Unpack(resp); err != nil {
		t.Error(err)
	}
	if len(msg.Questions) != 1 {
		t.Errorf("Wrong number of questions: %q", msg.Questions)
	}
	if len(msg.Answers) != 2 {
		t.Errorf("Wrong number of answers: %q", msg.Answers)
	}
}
