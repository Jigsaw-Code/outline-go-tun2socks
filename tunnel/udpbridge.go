package tunnel

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

func udpAddr(addr net.Addr) (*net.UDPAddr, error) {
	if udp, ok := addr.(*net.UDPAddr); ok {
		return udp, nil
	}
	return net.ResolveUDPAddr(addr.Network(), addr.String())
}

type nat struct {
	sync.RWMutex
	timeout  time.Duration
	m        map[tcpip.FullAddress]net.PacketConn
	listener core.UDPListener
	stack    *stack.Stack
}

type reverseUDPConn struct {
	localAddr tcpip.FullAddress
	stack     *stack.Stack
	n         *nat
}

func (c reverseUDPConn) WriteFrom(data []byte, addr *net.UDPAddr) error {
	// Based on sendUDP() in https://github.com/google/gvisor/blob/master/pkg/tcpip/transport/udp/endpoint.go#

	srcIP := tcpip.Address(string(addr.IP))
	protocol := header.IPv6ProtocolNumber
	if ip4 := srcIP.To4(); len(ip4) > 0 {
		srcIP = ip4
		protocol = header.IPv4ProtocolNumber
	}
	route, err := c.stack.FindRoute(1, srcIP, c.localAddr.Addr, protocol, false)
	if err != nil {
		return errors.New(err.String())
	}

	// WritePacket takes ownership of pkt.
	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: header.UDPMinimumSize + int(route.MaxHeaderLength()),
	})

	// Initialize the UDP header.
	udpHeader := header.UDP(pkt.TransportHeader().Push(header.UDPMinimumSize))
	pkt.TransportProtocolNumber = udp.ProtocolNumber

	// Copy packet contents
	pkt.Data().AppendView(buffer.NewViewFromBytes(data))
	length := uint16(pkt.Size())
	udpHeader.SetSourcePort(uint16(addr.Port))
	udpHeader.SetDestinationPort(c.localAddr.Port)
	udpHeader.SetLength(length)
	xsum := route.PseudoHeaderChecksum(udp.ProtocolNumber, length)
	xsum = header.Checksum(data, xsum)
	udpHeader.SetChecksum(^udpHeader.CalculateChecksum(xsum))
	headerParams := stack.NetworkHeaderParams{
		Protocol: udp.ProtocolNumber,
		TTL:      route.DefaultTTL(),
	}
	if err := route.WritePacket(headerParams, pkt); err != nil {
		log.Printf("UDP download write error: %v", err)
		return errors.New(err.String())
	}
	return nil
}

func (c reverseUDPConn) Close() error {
	c.n.Lock()
	delete(c.n.m, c.localAddr)
	c.n.Unlock()
	log.Printf("Closing UDP socket for %v", c.localAddr)
	return nil
}

func (n *nat) getConn(localAddr tcpip.FullAddress, netstack *stack.Stack) (net.PacketConn, error) {
	n.Lock()
	defer n.Unlock()
	conn, ok := n.m[localAddr]
	if !ok {
		reverse := reverseUDPConn{localAddr, n.stack, n}
		var err error
		conn, err = n.listener.ListenUDP(reverse)
		if err != nil {
			return nil, fmt.Errorf("failed to listen in NAT: %w", err)
		}
		n.m[localAddr] = conn
		go copyDownstream(reverse, conn)
	}
	return conn, nil
}

func copyDownstream(dst reverseUDPConn, src net.PacketConn) {
	buf := make([]byte, 2048) // TODO: Use a pool.
	for {
		n, addr, err := src.ReadFrom(buf)
		if err != nil && n == 0 {
			break
		}
		udp, err := udpAddr(addr)
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

func (n *nat) HandleOutboundPacket(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
	srcAddr := tcpip.FullAddress{
		Addr: id.RemoteAddress,
		Port: id.RemotePort,
	}
	dstAddr := net.UDPAddr{
		IP:   []byte(id.LocalAddress),
		Port: int(id.LocalPort),
	}
	conn, err := n.getConn(srcAddr, n.stack)
	if err != nil {
		log.Printf("UDP upload error: %v", err)
		return false
	}
	// AsView() can trigger reallocation if there are multiple segments in the range.
	// TODO: Check if this ever happens, and if so, make the Shadowsocks-UDP writer accept
	// multiple segments to concatenate before encryption.
	if _, err := conn.WriteTo(pkt.Data().AsRange().AsView(), &dstAddr); err != nil {
		log.Printf("UDP upload write error: %v", err)
		return false
	}
	conn.SetDeadline(time.Now().Add(n.timeout))
	return true
}

func MakeUDPBridge(netstack *stack.Stack, listener core.UDPListener, timeout time.Duration) *nat {
	return &nat{stack: netstack, timeout: timeout, m: make(map[tcpip.FullAddress]net.PacketConn), listener: listener}
}
