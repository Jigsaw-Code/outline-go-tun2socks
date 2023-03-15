package shadowsocks

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/eycorsican/go-tun2socks/core"
)

type udpHandler struct {
	// Protects the connections map
	sync.Mutex

	// Used to establish connections to the proxy
	listener onet.PacketListener

	// How long to wait for a packet from the proxy. Longer than this and the connection
	// is closed.
	timeout time.Duration

	// Maps connections from TUN to connections to the proxy.
	conns map[core.UDPConn]net.PacketConn
}

// NewUDPHandler returns a Shadowsocks UDP connection handler.
//
// `client` provides the Shadowsocks functionality.
// `timeout` is the UDP read and write timeout.
func NewUDPHandler(dialer onet.PacketListener, timeout time.Duration) core.UDPConnHandler {
	return &udpHandler{
		listener: dialer,
		timeout:  timeout,
		conns:    make(map[core.UDPConn]net.PacketConn, 8),
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	proxyConn, err := h.listener.ListenPacket(context.Background())
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = proxyConn
	h.Unlock()
	go h.handleDownstreamUDP(conn, proxyConn)
	return nil
}

func (h *udpHandler) handleDownstreamUDP(conn core.UDPConn, proxyConn net.PacketConn) {
	buf := core.NewBytes(core.BufSize)
	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()
	for {
		proxyConn.SetDeadline(time.Now().Add(h.timeout))
		n, addr, err := proxyConn.ReadFrom(buf)
		if err != nil {
			return
		}
		// No resolution will take place, the address sent by the proxy is a resolved IP.
		udpAddr, err := net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return
		}
		_, err = conn.WriteFrom(buf[:n], udpAddr)
		if err != nil {
			return
		}
	}
}

func (h *udpHandler) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	h.Lock()
	proxyConn, ok := h.conns[conn]
	h.Unlock()
	if !ok {
		return fmt.Errorf("connection %v->%v does not exist", conn.LocalAddr(), addr)
	}
	proxyConn.SetDeadline(time.Now().Add(h.timeout))
	_, err := proxyConn.WriteTo(data, addr)
	return err
}

func (h *udpHandler) Close(conn core.UDPConn) {
	conn.Close()
	h.Lock()
	defer h.Unlock()
	if proxyConn, ok := h.conns[conn]; ok {
		proxyConn.Close()
	}
}
