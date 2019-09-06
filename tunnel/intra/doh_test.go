package intra

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
	"testing"
)

var testURL = "https://dns.google/dns-query"
var ips = []string{
	"8.8.8.8",
	"8.8.4.4",
	"2001:4860:4860::8888",
	"2001:4860:4860::8844",
}
var parsedURL *url.URL

func init() {
	parsedURL, _ = url.Parse(testURL)
}

// Check that the constructor works.
func TestNewTransport(t *testing.T) {
	_, err := NewDoHTransport(testURL, ips, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// Check that the constructor rejects unsupported URLs.
func TestBadUrl(t *testing.T) {
	_, err := NewDoHTransport("ftp://www.example.com", nil, nil)
	if err == nil {
		t.Error("Expected error")
	}
	_, err = NewDoHTransport("https://www.example", nil, nil)
	if err == nil {
		t.Error("Expected error")
	}
}

// Check for failure when the query is too short to be valid.
func TestShortQuery(t *testing.T) {
	var qerr *queryError
	doh, _ := NewDoHTransport(testURL, ips, nil)
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

	doh, err := NewDoHTransport(testURL, ips, nil)
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
	doh, _ := NewDoHTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt
	go doh.Query([]byte{1, 2, 3, 4, 5})
	req := <-rt.req
	if req.URL.String() != testURL {
		t.Errorf("URL mismatch: %s != %s", req.URL.String(), testURL)
	}
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Error(err)
	}
	// The first two bytes are the ID, so they should be set to zero.
	if !bytes.Equal([]byte{0, 0, 3, 4, 5}, reqBody) {
		t.Errorf("Unexpected request body %v", reqBody)
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

// Check that a DOH response is returned correctly.
func TestResponse(t *testing.T) {
	doh, _ := NewDoHTransport(testURL, ips, nil)
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
		w.Write([]byte{0, 0, 8, 9, 10})
		w.Close()
	}()

	resp, err := doh.Query([]byte{1, 2, 3, 4, 5})
	if err != nil {
		t.Error(err)
	}
	// Query() should reconstitute the query ID in the response.
	if !bytes.Equal([]byte{1, 2, 8, 9, 10}, resp) {
		t.Errorf("Unexpected response %v", resp)
	}
}

// Simulate an empty response.  (This is not a compliant server
// behavior.)
func TestEmptyResponse(t *testing.T) {
	doh, _ := NewDoHTransport(testURL, ips, nil)
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

	_, err := doh.Query([]byte{1, 2, 3, 4, 5})
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
	doh, _ := NewDoHTransport(testURL, ips, nil)
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

	_, err := doh.Query([]byte{1, 2, 3, 4, 5})
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
	doh, _ := NewDoHTransport(testURL, ips, nil)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	rt.err = errors.New("test")
	_, err := doh.Query([]byte{1, 2, 3, 4, 5})
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
	DNSListener
	summary *DNSSummary
}

func (l *fakeListener) OnDNSTransaction(s *DNSSummary) {
	l.summary = s
}

type fakeConn struct {
	net.TCPConn
	remoteAddr fakeAddr
}

func (c *fakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

type fakeAddr struct {
	addr string
}

func (a fakeAddr) String() string {
	return a.addr
}

func (a fakeAddr) Network() string {
	return "tcp"
}

// Check that the DNSListener is called with a correct summary.
func TestListener(t *testing.T) {
	listener := &fakeListener{}
	doh, _ := NewDoHTransport(testURL, ips, listener)
	transport := doh.(*transport)
	rt := makeTestRoundTripper()
	transport.client.Transport = rt

	go func() {
		req := <-rt.req
		trace := httptrace.ContextClientTrace(req.Context())
		trace.GotConn(httptrace.GotConnInfo{Conn: &fakeConn{remoteAddr: fakeAddr{"foo:443"}}})

		r, w := io.Pipe()
		rt.resp <- &http.Response{
			StatusCode: 200,
			Body:       r,
			Request:    &http.Request{URL: parsedURL},
		}
		w.Write([]byte{0, 0, 8, 9, 10})
		w.Close()
	}()

	doh.Query([]byte{1, 2, 3, 4, 5})
	s := listener.summary
	if s.Latency < 0 {
		t.Errorf("Negative latency: %f", s.Latency)
	}
	if !bytes.Equal(s.Query, []byte{1, 2, 3, 4, 5}) {
		t.Errorf("Wrong query: %v", s.Query)
	}
	if !bytes.Equal(s.Response, []byte{1, 2, 8, 9, 10}) {
		t.Errorf("Wrong response: %v", s.Response)
	}
	if s.Server != "foo" {
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
	DNSTransport
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
	queryData := []byte{1, 2, 3, 4, 5}
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
	queryData := []byte{1, 2, 3, 4, 5}
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
	queryData := []byte{1, 2, 3, 4, 5}
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
	queryData := []byte{1, 2, 3, 4, 5}
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
