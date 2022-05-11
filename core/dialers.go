package core

import (
	"io"
	"net"
)

type DoubleStream interface {
	io.Reader
	io.Writer
	CloseRead() error
	CloseWrite() error
}

type TCPDialer interface {
	DialTCP(addr *net.TCPAddr) (DoubleStream, error)
}

type doublePipe struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (p doublePipe) Read(buf []byte) (int, error) {
	return p.reader.Read(buf)
}

func (p doublePipe) Write(buf []byte) (int, error) {
	return p.writer.Write(buf)
}

func (p doublePipe) CloseRead() error {
	return p.reader.Close()
}

func (p doublePipe) CloseWrite() error {
	return p.writer.Close()
}

func DoublePipe() (DoubleStream, DoubleStream) {
	upReader, upWriter := io.Pipe()
	downReader, downWriter := io.Pipe()
	top := doublePipe{upReader, downWriter}
	bottom := doublePipe{downReader, upWriter}
	return top, bottom
}

type ReverseUDPConn interface {
	WriteFrom(pkt []byte, src *net.UDPAddr) error
}

type UDPListener interface {
	ListenUDP(reverse ReverseUDPConn) (net.PacketConn, error)
}
