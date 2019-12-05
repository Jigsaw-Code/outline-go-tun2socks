package protect

import (
	"net"
	"syscall"
	"testing"
)

// The fake protector just records the file descriptors it was given.
type fakeProtector struct {
	Protector
	fds []int32
}

func (p *fakeProtector) Protect(fd int32) bool {
	p.fds = append(p.fds, fd)
	return true
}

// This interface serves as a supertype of net.TCPConn and net.UDPConn, so
// that they can share the verifyMatch() function.
type hasSyscallConn interface {
	SyscallConn() (syscall.RawConn, error)
}

func verifyMatch(t *testing.T, conn hasSyscallConn, p *fakeProtector) {
	rawconn, err := conn.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	rawconn.Control(func(fd uintptr) {
		if len(p.fds) == 0 {
			t.Fatalf("No file descriptors")
		}
		if int32(fd) != p.fds[0] {
			t.Fatalf("File descriptor mismatch: %d != %d", fd, p.fds[0])
		}
	})
}

func TestDialer(t *testing.T) {
	p := &fakeProtector{}
	d := dialer(p)
	if d.Control == nil {
		t.Errorf("Control function is nil")
	}
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	go l.Accept()
	conn, err := d.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	verifyMatch(t, conn.(*net.TCPConn), p)
	l.Close()
	conn.Close()
}

func TestDialTCP(t *testing.T) {
	p := &fakeProtector{}
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	go l.Accept()
	tcpaddr, err := net.ResolveTCPAddr(l.Addr().Network(), l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialTCP(p, tcpaddr)
	verifyMatch(t, conn, p)
	l.Close()
	conn.Close()
}

func TestListenUDP(t *testing.T) {
	p := &fakeProtector{}
	udpaddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := listenUDP(p, udpaddr)
	verifyMatch(t, conn, p)
	conn.Close()
}

func TestLookupIPAddr(t *testing.T) {
	p := &fakeProtector{}
	lookupIPAddr(p, "foo.test.")
	// Verify that Protect was called.
	if len(p.fds) == 0 {
		t.Fatal("Protect was not called")
	}
}
