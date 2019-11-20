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
	OptDefaultPaddingLen   = 400
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
// excluding any RFC7830 padding option.
func computePaddingSize(msgLen int, hasOptRr bool, blockSize int) int {
	var padSize int = 0
	// We'll always be adding a new padding header inside the OPT
	// RR's data.
	var extraPadding = kOptPaddingHeaderLen

	// If we don't already have an OPT RR, we'll need to add its
	// header as well!
	if !hasOptRr {
		extraPadding += kOptRrHeaderLen
	}

	for (msgLen+extraPadding+padSize)%blockSize != 0 {
		padSize++
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

	if optRes == nil {
		// Append a new OPT resource.
		msg.Additionals = append(msg.Additionals, dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  dnsmessage.MustNewName("."),
				Class: dnsmessage.ClassINET,
				TTL:   0,
			},
			Body: &dnsmessage.OPTResource{
				Options: []dnsmessage.Option{
					getPadding(len(rawMsg), false),
				},
			},
		})
	} else {
		// Search for a padding Option and delete it.
		for i, option := range optRes.Options {
			if option.Code == OptResourcePaddingCode {
				optRes.Options = append(optRes.Options[:i], optRes.Options[i+1:]...)
				break
			}
		}
		repackedMsg, err := msg.Pack()
		if err != nil {
			return nil, err
		}
		optRes.Options = append(optRes.Options, getPadding(len(repackedMsg), true))
	}

	// Re-pack the message, with compression unconditionally enabled.
	return msg.Pack()
}
