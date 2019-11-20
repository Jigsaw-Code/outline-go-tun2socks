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
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"reflect"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

var testURL = "https://dns.google/dns-query"
var ips = []string{
	"8.8.8.8",
	"8.8.4.4",
	"2001:4860:4860::8888",
	"2001:4860:4860::8844",
}
var parsedURL *url.URL

var testQuery dnsmessage.Message = dnsmessage.Message{
	Header: dnsmessage.Header{
		ID:                 0xbeef,
		Response:           true,
		OpCode:             0,
		Authoritative:      false,
		Truncated:          true,
		RecursionDesired:   false,
		RecursionAvailable: false,
		RCode:              0,
	},
	Questions: []dnsmessage.Question{
		dnsmessage.Question{
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

var testQueryBytes []byte = mustPack(&testQuery)

func init() {
	parsedURL, _ = url.Parse(testURL)
}

// Check that the constructor works.
func TestNewTransport(t *testing.T) {
	_, err := NewTransport(testURL, ips, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// Check that the constructor rejects unsupported URLs.
func TestBadUrl(t *testing.T) {
	_, err := NewTransport("ftp://www.example.com", nil, nil)
	if err == nil {
		t.Error("Expected error")
	}
	_, err = NewTransport("https://www.example", nil, nil)
	if err == nil {
		t.Error("Expected error")
	}
}

// Check for failure when the query is too short to be valid.
func TestShortQuery(t *testing.T) {
	var qerr *queryError
	doh, _ := NewTransport(testURL, ips, nil)
	_, err := doh.Query([]byte{})
	if err == nil {
		t.Error("Empty query should fail")
	} else if !errors.As(err, &qerr) {
		t.Errorf("Wrong error type: %v", err)
	} else if qerr.status != BadQuery {
		t.Errorf("Wrong error status: %d", qerr.status)
	}

	_, err = doh.Query([]byte{1})
	if err == nil {
		t.Error("One byte query should fail")
	} else if !errors.As(err, &qerr) {
		t.Errorf("Wrong error type: %v", err)
	} else if qerr.status != BadQuery {
		t.Errorf("Wrong error status: %d", qerr.status)
	}
}

// Send a DoH query to an actual DoH server
func TestQueryIntegration(t *testing.T) {
	queryData := []byte{
		111, 222, // [0-1]   query ID
		1, 0, // [2-3]   flags, RD=1
		0, 1, // [4-5]   QDCOUNT (number of queries) = 1
		0, 0, // [6-7]   ANCOUNT (number of answers) = 0
		0, 0, // [8-9]   NSCOUNT (number of authoritative answers) = 0
		0, 0, // [10-11] ARCOUNT (number of additional records) = 0
		// Start of first query
		7, 'y', 'o', 'u', 't', 'u', 'b', 'e',
		3, 'c', 'o', 'm',
		0,    // null terminator of FQDN (DNS root)
		0, 1, // QTYPE = A
		0, 1, // QCLASS = IN (Internet)
	}

	doh, err := NewTransport(testURL, ips, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err2 := doh.Query(queryData)
	if err2 != nil {
		t.Fatal(err2)
	}
	if resp[0] != queryData[0] || resp[1] != queryData[1] {
		t.Error("Query ID mismatch")
	}
	if len(resp) <= len(queryData) {
		t.Error("Response is short")
	}
}

type testRoundTripper struct {
	http.RoundTripper
	req  chan *http.Request
	resp chan *http.Response
	err  error
}

func makeTestRoundTripper() *testRoundTripper {
	return &testRoundTripper{
		req:  make(chan *http.Request),
		resp: make(chan *http.Response),
	}
}

func (r *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.req <- req
	return <-r.resp, nil
}

// Check that a DNS query is converted correctly into an HTTP query.
func TestRequest(t *testing.T) {
	doh, _ := NewTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt
	go doh.Query(testQueryBytes)
	req := <-rt.req
	if req.URL.String() != testURL {
		t.Errorf("URL mismatch: %s != %s", req.URL.String(), testURL)
	}
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Error(err)
	}
	// Parse reqBody into a Message.
	var newQuery dnsmessage.Message
	newQuery.Unpack(reqBody)
	// Ensure the converted request has an ID of zero.
	if newQuery.Header.ID != 0 {
		t.Errorf("Unexpected request header id: %v", newQuery.Header.ID)
	}
	// Check that all fields except for Header.ID and Additionals
	// are the same as the original.  Additionals may differ if
	// padding was added.
	if !queriesMostlyEqual(testQuery, newQuery) {
		t.Errorf("Unexpected query body:\n\t%v\nExpected:\n\t%v", newQuery, testQuery)
	}
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/dns-message" {
		t.Errorf("Wrong content type: %s", contentType)
	}
	accept := req.Header.Get("Accept")
	if accept != "application/dns-message" {
		t.Errorf("Wrong Accept header: %s", accept)
	}
}

// Check that all fields of m1 match those of m2, except for Header.ID
// and Additionals.
func queriesMostlyEqual(m1 dnsmessage.Message, m2 dnsmessage.Message) bool {
	// Make fields we don't care about match, so that equality check is easy.
	m1.Header.ID = m2.Header.ID
	m1.Additionals = m2.Additionals
	return reflect.DeepEqual(m1, m2)
}

// Check that a DOH response is returned correctly.
func TestResponse(t *testing.T) {
	doh, _ := NewTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	// Fake server.
	go func() {
		<-rt.req
		r, w := io.Pipe()
		rt.resp <- &http.Response{
			StatusCode: 200,
			Body:       r,
			Request:    &http.Request{URL: parsedURL},
		}
		// The DOH response should have a zero query ID.
		var modifiedQuery dnsmessage.Message = testQuery
		modifiedQuery.Header.ID = 0
		w.Write(mustPack(&modifiedQuery))
		w.Close()
	}()

	resp, err := doh.Query(testQueryBytes)
	if err != nil {
		t.Error(err)
	}

	// Parse the response as a DNS message.
	var respParsed dnsmessage.Message
	if err := respParsed.Unpack(resp); err != nil {
		t.Errorf("Could not parse Message %v", err)
	}

	// Query() should reconstitute the query ID in the response.
	if respParsed.Header.ID != testQuery.Header.ID ||
		!queriesMostlyEqual(respParsed, testQuery) {
		t.Errorf("Unexpected response %v", resp)
	}
}

// Simulate an empty response.  (This is not a compliant server
// behavior.)
func TestEmptyResponse(t *testing.T) {
	doh, _ := NewTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	// Fake server.
	go func() {
		<-rt.req
		// Make an empty body.
		r, w := io.Pipe()
		w.Close()
		rt.resp <- &http.Response{
			StatusCode: 200,
			Body:       r,
			Request:    &http.Request{URL: parsedURL},
		}
	}()

	_, err := doh.Query(testQueryBytes)
	var qerr *queryError
	if err == nil {
		t.Error("Empty body should cause an error")
	} else if !errors.As(err, &qerr) {
		t.Errorf("Wrong error type: %v", err)
	} else if qerr.status != BadResponse {
		t.Errorf("Wrong error status: %d", qerr.status)
	}
}

// Simulate a non-200 HTTP response code.
func TestHTTPError(t *testing.T) {
	doh, _ := NewTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	go func() {
		<-rt.req
		r, w := io.Pipe()
		rt.resp <- &http.Response{
			StatusCode: 500,
			Body:       r,
			Request:    &http.Request{URL: parsedURL},
		}
		w.Write([]byte{0, 0, 8, 9, 10})
		w.Close()
	}()

	_, err := doh.Query(testQueryBytes)
	var qerr *queryError
	if err == nil {
		t.Error("Empty body should cause an error")
	} else if !errors.As(err, &qerr) {
		t.Errorf("Wrong error type: %v", err)
	} else if qerr.status != HTTPError {
		t.Errorf("Wrong error status: %d", qerr.status)
	}
}

// Simulate an HTTP query error.
func TestSendFailed(t *testing.T) {
	doh, _ := NewTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	rt.err = errors.New("test")
	_, err := doh.Query(testQueryBytes)
	var qerr *queryError
	if err == nil {
		t.Error("Send failure should be reported")
	} else if !errors.As(err, &qerr) {
		t.Errorf("Wrong error type: %v", err)
	} else if qerr.status != SendFailed {
		t.Errorf("Wrong error status: %d", qerr.status)
	} else if !errors.Is(qerr, rt.err) {
		t.Errorf("Underlying error is not retained")
	}
}

type fakeListener struct {
	Listener
	summary *Summary
}

func (l *fakeListener) OnQuery(url string) Token {
	return nil
}

func (l *fakeListener) OnResponse(tok Token, summ *Summary) {
	l.summary = summ
}

type fakeConn struct {
	net.TCPConn
	remoteAddr *net.TCPAddr
}

func (c *fakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Check that the DNSListener is called with a correct summary.
func TestListener(t *testing.T) {
	listener := &fakeListener{}
	doh, _ := NewTransport(testURL, ips, listener)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	go func() {
		req := <-rt.req
		trace := httptrace.ContextClientTrace(req.Context())
		trace.GotConn(httptrace.GotConnInfo{
			Conn: &fakeConn{
				remoteAddr: &net.TCPAddr{
					IP:   net.ParseIP("192.0.2.2"),
					Port: 443,
				}}})

		r, w := io.Pipe()
		rt.resp <- &http.Response{
			StatusCode: 200,
			Body:       r,
			Request:    &http.Request{URL: parsedURL},
		}
		w.Write([]byte{0, 0, 8, 9, 10})
		w.Close()
	}()

	doh.Query(testQueryBytes)
	s := listener.summary
	if s.Latency < 0 {
		t.Errorf("Negative latency: %f", s.Latency)
	}
	if !bytes.Equal(s.Query, testQueryBytes) {
		t.Errorf("Wrong query: %v", s.Query)
	}
	if !bytes.Equal(s.Response, []byte{0xbe, 0xef, 8, 9, 10}) {
		t.Errorf("Wrong response: %v", s.Response)
	}
	if s.Server != "192.0.2.2" {
		t.Errorf("Wrong server IP string: %s", s.Server)
	}
	if s.Status != Complete {
		t.Errorf("Wrong status: %d", s.Status)
	}
}

type socket struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (c *socket) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *socket) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

func (c *socket) Close() error {
	e1 := c.r.Close()
	e2 := c.w.Close()
	if e1 != nil {
		return e1
	}
	return e2
}

func makePair() (io.ReadWriteCloser, io.ReadWriteCloser) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &socket{r1, w2}, &socket{r2, w1}
}

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
	client, server := makePair()

	// Start the forwarder running.
	go Accept(doh, server)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := testQueryBytes
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
	n, err = client.Read(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(responseData, resp) {
		t.Error("Response mismatch")
	}

	client.Close()
}

// Sends a TCP query that results in failure.  When a query fails,
// Accept should close the TCP socket.
func TestAcceptFail(t *testing.T) {
	doh := newFakeTransport()
	client, server := makePair()

	// Start the forwarder running.
	go Accept(doh, server)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := testQueryBytes
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
	client, server := makePair()

	// Start the forwarder running.
	go Accept(doh, server)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := testQueryBytes
	binary.BigEndian.PutUint16(lbuf, uint16(len(queryData)))
	client.Write(lbuf)
	client.Write(queryData)

	// Read query
	queryRead := <-doh.query
	if !bytes.Equal(queryRead, queryData) {
		t.Error("Query mismatch")
	}

	// Close the TCP connection
	client.Close()

	// Send fake response too late.
	responseData := []byte{1, 2, 8, 9, 10}
	doh.response <- responseData
}

// Test failure due to a response that is larger than the
// maximum message size for DNS over TCP (65535).
func TestAcceptOversize(t *testing.T) {
	doh := newFakeTransport()
	client, server := makePair()

	// Start the forwarder running.
	go Accept(doh, server)

	lbuf := make([]byte, 2)
	// Send Query
	queryData := testQueryBytes
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
