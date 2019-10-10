package intra

import (
	"io"
	"net"
)

type splitter struct {
	*net.TCPConn
	used bool // Initially false.  Becomes true after the first write.
}

// DialWithSplit returns a TCP connection that always splits the initial upstream segment.
// Like net.Conn, it is intended for two-threaded use, with one thread calling
// Read and CloseRead, and another calling Write, ReadFrom, and CloseWrite.
func DialWithSplit(addr *net.TCPAddr) (DuplexConn, error) {
	conn, err := net.DialTCP(addr.Network(), nil, addr)
	if err != nil {
		return nil, err
	}

	return &splitter{TCPConn: conn}, nil
}

// Write-related functions
func (s *splitter) Write(b []byte) (int, error) {
	conn := s.TCPConn
	if s.used {
		// After the first write, there is no special write behavior.
		return conn.Write(b)
	}

	// Setting `used` to true ensures that this code only runs once per socket.
	s.used = true
	b1, b2 := splitHello(b)
	n1, err := conn.Write(b1)
	if err != nil {
		return n1, err
	}
	n2, err := conn.Write(b2)
	return n1 + n2, err
}

func (s *splitter) ReadFrom(reader io.Reader) (bytes int64, err error) {
	if !s.used {
		// This is the first write on this socket.
		// Use copyOnce(), which calls Write(), to get Write's splitting behavior for
		// the first segment.
		if bytes, err = copyOnce(s, reader); err != nil {
			return
		}
	}

	var b int64
	b, err = s.TCPConn.ReadFrom(reader)
	bytes += b
	return
}
