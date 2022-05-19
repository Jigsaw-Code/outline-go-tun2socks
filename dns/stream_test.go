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
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

var simpleQuery dnsmessage.Message = dnsmessage.Message{
	Header: dnsmessage.Header{
		ID:                 0xbeef,
		Response:           false,
		OpCode:             0,
		Authoritative:      false,
		Truncated:          false,
		RecursionDesired:   true,
		RecursionAvailable: false,
		RCode:              0,
	},
	Questions: []dnsmessage.Question{
		{
			Name:  dnsmessage.MustNewName("www.example.com."),
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}},
	Answers:     []dnsmessage.Resource{},
	Authorities: []dnsmessage.Resource{},
	Additionals: []dnsmessage.Resource{},
}

func mustPack(m *dnsmessage.Message) []byte {
	packed, err := m.Pack()
	if err != nil {
		panic(err)
	}
	return packed
}

func mustUnpack(q []byte) *dnsmessage.Message {
	var m dnsmessage.Message
	err := m.Unpack(q)
	if err != nil {
		panic(err)
	}
	return &m
}

var simpleQueryBytes []byte = mustPack(&simpleQuery)

type fakeTransport struct {
	Transport
	query    chan []byte
	response chan []byte
	err      error
}

func (t *fakeTransport) Query(q []byte) ([]byte, error) {
	t.query <- q
	if t.err != nil {
		return nil, t.err
	}
	return <-t.response, nil
}

func (t *fakeTransport) GetURL() string {
	return "fake"
}

func (t *fakeTransport) Close() {
	t.err = errors.New("closed")
	close(t.query)
	close(t.response)
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		query:    make(chan []byte),
		response: make(chan []byte),
	}
}

// Test a successful query over TCP
func TestAccept(t *testing.T) {
	doh := newFakeTransport()

	// Start the forwarder running.
	client := Accept(doh)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := simpleQueryBytes
	binary.BigEndian.PutUint16(lbuf, uint16(len(queryData)))
	n, err := client.Write(lbuf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Error("Length write problem")
	}
	n, err = client.Write(queryData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(queryData) {
		t.Error("Query write problem")
	}

	// Read query
	queryRead := <-doh.query
	if !bytes.Equal(queryRead, queryData) {
		t.Error("Query mismatch")
	}

	// Send fake response
	responseData := []byte{1, 2, 8, 9, 10}
	doh.response <- responseData

	// Get Response
	n, err = client.Read(lbuf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Error("Length read problem")
	}
	rlen := binary.BigEndian.Uint16(lbuf)
	resp := make([]byte, int(rlen))
	_, err = client.Read(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(responseData, resp) {
		t.Error("Response mismatch")
	}

	client.CloseWrite()
}

// Sends a TCP query that results in failure.  When a query fails,
// Accept should close the TCP socket.
func TestAcceptFail(t *testing.T) {
	doh := newFakeTransport()

	// Start the forwarder running.
	client := Accept(doh)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := simpleQueryBytes
	binary.BigEndian.PutUint16(lbuf, uint16(len(queryData)))
	client.Write(lbuf)
	client.Write(queryData)

	// Indicate that the query failed
	doh.err = errors.New("fake error")

	// Read query
	queryRead := <-doh.query
	if !bytes.Equal(queryRead, queryData) {
		t.Error("Query mismatch")
	}

	// Accept should have closed the socket.
	n, _ := client.Read(lbuf)
	if n != 0 {
		t.Error("Expected to read 0 bytes")
	}
}

// Sends a TCP query, and closes the socket before the response is sent.
// This tests for crashes when a response cannot be delivered.
func TestAcceptClose(t *testing.T) {
	doh := newFakeTransport()

	// Start the forwarder running.
	client := Accept(doh)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := simpleQueryBytes
	binary.BigEndian.PutUint16(lbuf, uint16(len(queryData)))
	client.Write(lbuf)
	client.Write(queryData)

	// Read query
	queryRead := <-doh.query
	if !bytes.Equal(queryRead, queryData) {
		t.Error("Query mismatch")
	}

	// Close the TCP connection
	client.CloseWrite()

	// Send fake response too late.
	responseData := []byte{1, 2, 8, 9, 10}
	doh.response <- responseData
}

// Test failure due to a response that is larger than the
// maximum message size for DNS over TCP (65535).
func TestAcceptOversize(t *testing.T) {
	doh := newFakeTransport()

	// Start the forwarder running.
	client := Accept(doh)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := simpleQueryBytes
	binary.BigEndian.PutUint16(lbuf, uint16(len(queryData)))
	client.Write(lbuf)
	client.Write(queryData)

	// Read query
	<-doh.query

	// Send oversize response
	doh.response <- make([]byte, 65536)

	// Accept should have closed the socket because the response
	// cannot be written.
	n, _ := client.Read(lbuf)
	if n != 0 {
		t.Error("Expected to read 0 bytes")
	}
}
