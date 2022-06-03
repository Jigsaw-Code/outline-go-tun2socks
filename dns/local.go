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
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// FIXME: We have an Ouroboros problem on Linux and Windows.  There is no escape
// route for the local DNS server.

// FIXME: This will not work on Linux or Windows.
func GetResolverIP() (net.IP, error) {
	var resolverAddress string
	resolver := net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			resolverAddress = address
			return nil, errors.New("dummy resolver")
		},
	}
	resolver.LookupHost(context.TODO(), "example.com")
	udpAddr, err := net.ResolveUDPAddr("udp", resolverAddress)
	if err != nil {
		return nil, err
	}
	return udpAddr.IP, nil
}

type udpTransport struct {
	resolver net.UDPAddr
}

func (t udpTransport) Query(q []byte) ([]byte, error) {
	conn, err := net.DialUDP("udp", nil, &t.resolver)
	if err != nil {
		return TryServfail(q), err
	}
	if _, err := conn.Write(q); err != nil {
		return TryServfail(q), err
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// TODO: Use a pool
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return TryServfail(q), err
	}
	return buf[:n], nil
}

func (t udpTransport) GetURL() string {
	return t.resolver.String()
}

func MakeUDPTransport(resolver net.IP) Transport {
	return udpTransport{
		resolver: net.UDPAddr{
			IP:   resolver,
			Port: 53,
		},
	}
}

type tcpTransport struct {
	resolver net.TCPAddr
}

func (t tcpTransport) Query(q []byte) ([]byte, error) {
	// TODO: Reuse the TCP socket.
	conn, err := net.DialTCP("tcp", nil, &t.resolver)
	if err != nil {
		return TryServfail(q), err
	}
	queryWithHeader := make([]byte, len(q)+2)
	binary.BigEndian.PutUint16(queryWithHeader, uint16(len(q)))
	copy(queryWithHeader[2:], q)
	if _, err := conn.Write(queryWithHeader); err != nil {
		return TryServfail(q), err
	}
	conn.CloseWrite()
	// TODO: Use a pool
	responseHeader := make([]byte, 2)
	if _, err := io.ReadFull(conn, responseHeader); err != nil {
		return TryServfail(q), err
	}
	responseLength := binary.BigEndian.Uint16(responseHeader)
	// TODO: Use a pool
	buf := make([]byte, responseLength)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return TryServfail(q), err
	}
	return buf, nil
}

func (t tcpTransport) GetURL() string {
	return t.resolver.String()
}

func MakeTCPTransport(resolver net.IP) Transport {
	return tcpTransport{
		resolver: net.TCPAddr{
			IP:   resolver,
			Port: 53,
		},
	}
}

type localTransport struct{}

func (t localTransport) Query(q []byte) ([]byte, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(q); err != nil {
		return nil, err
	}
	if len(msg.Questions) == 0 {
		return nil, errors.New("query has no questions")
	}
	q0 := msg.Questions[0]
	if q0.Class != dnsmessage.ClassINET {
		return nil, errors.New("weird class")
	}
	header := dnsmessage.Header{
		ID:                 msg.ID,
		Response:           true,
		OpCode:             msg.OpCode,
		RecursionAvailable: true,
		RCode:              dnsmessage.RCodeSuccess,
	}
	builder := dnsmessage.NewBuilder(nil, header)
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(q0); err != nil {
		return nil, err
	}
	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}
	resourceHeader := dnsmessage.ResourceHeader{
		Name:  q0.Name,
		Type:  q0.Type,
		Class: q0.Class,
	}
	qname := q0.Name.String()
	switch q0.Type {
	case dnsmessage.TypeA:
		ips, err := net.LookupIP(qname)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if len(ip) == 4 {
				var resource dnsmessage.AResource
				copy(resource.A[:], ip[:])
				builder.AResource(resourceHeader, resource)
			}
		}
	case dnsmessage.TypeAAAA:
		ips, err := net.LookupIP(qname)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if len(ip) == 16 {
				var resource dnsmessage.AAAAResource
				copy(resource.AAAA[:], ip[:])
				builder.AAAAResource(resourceHeader, resource)
			}
		}
	case dnsmessage.TypeCNAME:
		//cname, err := net.LookupCNAME(name)
	case dnsmessage.TypeTXT:
		//txt, err := net.LookupTXT(name)
	case dnsmessage.TypeMX:
		//mx, err := net.LookupMX(name)
	case dnsmessage.TypeNS:
		//ns, err := net.LookupNS(name)
	case dnsmessage.TypeSRV:
		//cname, srvs, err := net.LookupSRV("", "", name)
	case dnsmessage.TypePTR:
		//ptrs, err := net.LookupAddr(name)
	default:
		return TryServfail(q), errors.New("unsupported qtype")
	}
	return builder.Finish()
}

func (t localTransport) GetURL() string {
	return "local DNS"
}

func MakeLocalTransport() Transport {
	return localTransport{}
}
