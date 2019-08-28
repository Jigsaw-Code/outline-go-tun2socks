package shadowsocks

import (
	"errors"
	"net"
	"testing"
	"time"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
)

func TestIsUDPSupported_Success(t *testing.T) {
	client := &fakeSSClient{}
	err := IsUDPSupported(client, shadowsocks.NewAddr("", ""))
	if err != nil {
		t.Fail()
	}
}

func TestIsUDPSupported_Fail(t *testing.T) {
	client := &fakeSSClient{failUDP: true}
	err := IsUDPSupported(client, shadowsocks.NewAddr("", ""))
	if err == nil {
		t.Fail()
	}
}

func TestCheckConnectivity_Success(t *testing.T) {
	client := &fakeSSClient{}
	r := CheckConnectivity(client)
	if !r.IsReachable || !r.IsAuthenticated || !r.IsUDPSupported {
		t.Fail()
	}
}

func TestCheckConnectivity_FailReachability(t *testing.T) {
	client := &fakeSSClient{failReachability: true}
	r := CheckConnectivity(client)
	if r.IsReachable || r.IsAuthenticated || r.IsUDPSupported {
		t.Fail()
	}
}

func TestCheckConnectivity_FailAuthentication(t *testing.T) {
	client := &fakeSSClient{failAuthentication: true}
	r := CheckConnectivity(client)
	if !r.IsReachable || r.IsAuthenticated || r.IsUDPSupported {
		t.Fail()
	}
}

func TestCheckConnectivity_FailUDP(t *testing.T) {
	client := &fakeSSClient{failUDP: true}
	r := CheckConnectivity(client)
	if !r.IsReachable || !r.IsAuthenticated || r.IsUDPSupported {
		t.Fail()
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
	failRead bool
}

func (c *fakePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return len(b), nil // Write always succeeds
}

func (c *fakePacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if c.failRead {
		return 0, nil, errors.New("Fake read error")
	}
	return len(b), nil, nil
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
