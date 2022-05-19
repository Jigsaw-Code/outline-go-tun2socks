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

package core

import (
	"context"
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

type BaseDialer interface {
	Dial(network, address string) (net.Conn, error)
}

type dialer struct {
	*net.Dialer
}

func (d dialer) DialTCP(dst *net.TCPAddr) (DoubleStream, error) {
	conn, err := d.Dial(dst.Network(), dst.String())
	if err != nil {
		return nil, err
	}
	return conn.(DoubleStream), nil
}

func MakeTCPDialer(d *net.Dialer) TCPDialer {
	return dialer{d}
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
	Close() error
}

type UDPListener interface {
	ListenUDP(reverse ReverseUDPConn) (net.PacketConn, error)
}

type DefaultUDPListener struct {
	*net.ListenConfig
}

func (l DefaultUDPListener) ListenUDP(_ ReverseUDPConn) (net.PacketConn, error) {
	return l.ListenPacket(context.TODO(), "udp", "")
}

func UDPAddr(addr net.Addr) (*net.UDPAddr, error) {
	if udp, ok := addr.(*net.UDPAddr); ok {
		return udp, nil
	}
	return net.ResolveUDPAddr(addr.Network(), addr.String())
}

func CopyDownstream(dst ReverseUDPConn, src net.PacketConn) {
	buf := make([]byte, 2048) // TODO: Use a pool.
	for {
		n, addr, err := src.ReadFrom(buf)
		if err != nil && n == 0 {
			break
		}
		udp, err := UDPAddr(addr)
		if err != nil {
			break
		}
		if err := dst.WriteFrom(buf[:n], udp); err != nil {
			break
		}
	}
	src.Close()
	dst.Close()
}
