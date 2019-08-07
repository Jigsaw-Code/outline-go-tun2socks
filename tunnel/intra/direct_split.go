package intra

import (
	"io"
	"net"
)

type splitter struct {
	DuplexConn
	conn  *net.TCPConn
	fresh bool
}

// DialWithSplit returns a TCP connection that always splits the initial upstream segment.
// Like net.Conn, it is intended for two-threaded use, with one thread calling
// Read and CloseRead, and another calling Write, ReadFrom, and CloseWrite.
func DialWithSplit(network string, addr *net.TCPAddr) (DuplexConn, error) {
	conn, err := net.DialTCP(network, nil, addr)
	if err != nil {
		return nil, err
	}

	r := &retrier{
		conn: conn,
	}

	return r, nil
}

// Read-related functions.
func (s *splitter) Read(buf []byte) (int, error) {
	return s.conn.Read(buf)
}

func (s *splitter) CloseRead() error {
	return s.conn.CloseRead()
}

// Write-related functions
func (s *splitter) Write(b []byte) (int, error) {
	if s.fresh {
		s.fresh = false
		b1, b2 := splitHello(b)
		n1, err := s.conn.Write(b1)
		if err != nil {
			return n1, err
		}
		n2, err := s.conn.Write(b2)
		return n1 + n2, err
	}
	return s.conn.Write(b)
}

func (s *splitter) ReadFrom(reader io.Reader) (bytes int64, err error) {
	if s.fresh {
		if bytes, err = copy(s, reader); err != nil {
			return
		}
	}

	var b int64
	b, err = s.conn.ReadFrom(reader)
	bytes += b
	return
}

func (s *splitter) CloseWrite() error {
	return s.conn.CloseWrite()
}
