package shadowsocks

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

func TestCheckUDPConnectivityWithDNS_Success(t *testing.T) {
	client := &fakeSSClient{}
	err := CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("", ""))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestCheckUDPConnectivityWithDNS_Fail(t *testing.T) {
	client := &fakeSSClient{failUDP: true}
	err := CheckUDPConnectivityWithDNS(client, shadowsocks.NewAddr("", ""))
	if err == nil {
		t.Fail()
	}
}

func TestCheckTCPConnectivityWithHTTP_Success(t *testing.T) {
	client := &fakeSSClient{}
	err := CheckTCPConnectivityWithHTTP(client, "")
	if err != nil {
		t.Fail()
	}
}

func TestCheckTCPConnectivityWithHTTP_FailReachability(t *testing.T) {
	client := &fakeSSClient{failReachability: true}
	err := CheckTCPConnectivityWithHTTP(client, "")
	if err == nil {
		t.Fail()
	}
	if _, ok := err.(*ReachabilityError); !ok {
		t.Fatalf("Expected reachability error, got: %v", reflect.TypeOf(err))
	}
}

func TestCheckTCPConnectivityWithHTTP_FailAuthentication(t *testing.T) {
	client := &fakeSSClient{failAuthentication: true}
	err := CheckTCPConnectivityWithHTTP(client, "")
	if err == nil {
		t.Fail()
	}
	if _, ok := err.(*AuthenticationError); !ok {
		t.Fatalf("Expected authentication error, got: %v", reflect.TypeOf(err))
	}
}

// Fake shadowsocks.Client that can be configured to return failing UDP and TCP connections.
type fakeSSClient struct {
	failReachability   bool
	failAuthentication bool
	failUDP            bool
}

func (c *fakeSSClient) DialTCP(laddr *net.TCPAddr, raddr string) (onet.DuplexConn, error) {
	if c.failReachability {
		return nil, &net.OpError{}
	}
	return &fakeDuplexConn{failRead: c.failAuthentication}, nil
}
func (c *fakeSSClient) ListenUDP(laddr *net.UDPAddr) (net.PacketConn, error) {
	conn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}
	// The UDP check should fail if any of the failure conditions are true since it is a superset of the others.
	failRead := c.failAuthentication || c.failUDP || c.failReachability
	return &fakePacketConn{PacketConn: conn, failRead: failRead}, nil
}

// Fake PacketConn that fails `ReadFrom` calls when `failRead` is true.
type fakePacketConn struct {
	net.PacketConn
	addr     net.Addr
	failRead bool
}

func (c *fakePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	c.addr = addr
	return len(b), nil // Write always succeeds
}

func (c *fakePacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if c.failRead {
		return 0, c.addr, errors.New("Fake read error")
	}
	return len(b), c.addr, nil
}

// Fake DuplexConn that fails `Read` calls when `failRead` is true.
type fakeDuplexConn struct {
	onet.DuplexConn
	failRead bool
}

func (c *fakeDuplexConn) Read(b []byte) (int, error) {
	if c.failRead {
		return 0, errors.New("Fake read error")
	}
	return len(b), nil
}

func (c *fakeDuplexConn) Write(b []byte) (int, error) {
	return len(b), nil // Write always succeeds
}

func (c *fakeDuplexConn) Close() error { return nil }

func (c *fakeDuplexConn) LocalAddr() net.Addr { return nil }

func (c *fakeDuplexConn) RemoteAddr() net.Addr { return nil }

func (c *fakeDuplexConn) SetDeadline(t time.Time) error { return nil }

func (c *fakeDuplexConn) SetReadDeadline(t time.Time) error { return nil }

func (c *fakeDuplexConn) SetWriteDeadline(t time.Time) error { return nil }

func (c *fakeDuplexConn) CloseRead() error { return nil }

func (c *fakeDuplexConn) CloseWrite() error { return nil }
