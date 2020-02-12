// Copyright 2020 The Outline Authors
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

package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"

	"github.com/Jigsaw-Code/outline-go-tun2socks/tunnel/intra/split"
)

func main() {
	if len(os.Args) < 2 {
		log.Printf("Usage: %s destination [SNI]", os.Args[0])
		log.Printf("This tool attempts TLS connection to the destination (port 443), with and without splitting.")
		log.Printf("If the SNI is specified, it overrides the destination, which can be an IP address.")
		return
	}
	destination := os.Args[1]
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(destination, "443"))
	if err != nil {
		log.Println("Couldn't resolve destination")
		return
	}
	sni := destination
	if len(os.Args) > 2 {
		sni = os.Args[2]
	}

	log.Println("Trying direct connection")
	conn, err := net.DialTCP(addr.Network(), nil, addr)
	if err != nil {
		log.Printf("Could not establish a TCP connection: %v", err)
		return
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: sni})
	err = tlsConn.Handshake()
	if err != nil {
		log.Printf("Direct TLS handshake failed: %v", err)
	} else {
		log.Printf("Direct TLS succeeded")
	}

	log.Println("Trying split connection")
	splitConn, err := split.DialWithSplit(&net.Dialer{}, addr)
	if err != nil {
		log.Printf("Could not establish a splitting socket: %v", err)
		return
	}
	tlsConn2 := tls.Client(splitConn, &tls.Config{ServerName: sni})
	err = tlsConn2.Handshake()
	if err != nil {
		log.Printf("Split TLS handshake failed: %v", err)
		return
	} else {
		log.Printf("Split TLS succeeded")
	}
}
