// Copyright 2019 The Outline Authors
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

package doh

import (
	"golang.org/x/net/dns/dnsmessage"
)

const (
	OptResourcePaddingCode = 12
	OptDefaultPaddingLen   = 128 // RFC8467 recommendation
)

const kOptRrHeaderLen int = 1 + // DOMAIN NAME
	2 + // TYPE
	2 + // CLASS
	4 + // TTL
	2 // RDLEN

const kOptPaddingHeaderLen int = 2 + // OPTION-CODE
	2 // OPTION-LENGTH

// Compute the number of padding bytes needed, excluding
// headers. Assumes that |msgLen| is the length of a raw DNS message
// excluding any RFC7830 padding option, and that the message is fully
// label-compressed.
func computePaddingSize(msgLen int, hasOptRr bool, blockSize int) int {
	// We'll always be adding a new padding header inside the OPT
	// RR's data.
	var extraPadding = kOptPaddingHeaderLen

	// If we don't already have an OPT RR, we'll need to add its
	// header as well!
	if !hasOptRr {
		extraPadding += kOptRrHeaderLen
	}

	var padSize int = blockSize - (msgLen+extraPadding)%blockSize
	if padSize < 0 {
		padSize *= -1
	}
	if padSize%blockSize == 0 {
		padSize = 0
	}
	return padSize
}

func getPadding(msgLen int, hasOptRr bool) dnsmessage.Option {
	optPadding := dnsmessage.Option{
		Code: OptResourcePaddingCode,
		Data: make([]byte, computePaddingSize(msgLen, hasOptRr, OptDefaultPaddingLen)),
	}
	return optPadding
}

// Add EDNS padding, as defined in RFC7830, to a raw DNS message.
func AddEdnsPadding(rawMsg []byte) ([]byte, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(rawMsg); err != nil {
		return nil, err
	}

	// Search for OPT resource and save |optRes| pointer if possible.
	var optRes *dnsmessage.OPTResource = nil
	for _, additional := range msg.Additionals {
		switch additional.Body.(type) {
		case *dnsmessage.OPTResource:
			optRes = additional.Body.(*dnsmessage.OPTResource)
			break
		}
	}

	if optRes != nil {
		// If the message already contains padding, we will
		// respect the application's padding.
		return rawMsg, nil
	}

	// Build the padding option that we will need. We can't use
	// the length of |rawMsg|, since its labels may be compressed
	// differently than the way the Pack function does it.
	compressedMsg, err := msg.Pack()
	if err != nil {
		return nil, err
	}
	paddingOption := getPadding(len(compressedMsg), false)

	// Append a new OPT resource.
	msg.Additionals = append(msg.Additionals, dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  dnsmessage.MustNewName("."),
			Class: dnsmessage.ClassINET,
			TTL:   0,
		},
		Body: &dnsmessage.OPTResource{
			Options: []dnsmessage.Option{paddingOption},
		},
	})

	// Re-pack the message, with compression unconditionally enabled.
	return msg.Pack()
}
