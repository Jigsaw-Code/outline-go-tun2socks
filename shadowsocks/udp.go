package shadowsocks

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
	"github.com/eycorsican/go-tun2socks/core"
)

type udpHandler struct {
	sync.Mutex

	client  shadowsocks.Client
	timeout time.Duration
	conns   map[core.UDPConn]net.PacketConn
}

// NewUDPHandler TODO
func NewUDPHandler(host string, port int, password, cipher string, timeout time.Duration) core.UDPConnHandler {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return nil
	}
	return &udpHandler{
		client:  client,
		timeout: timeout,
		conns:   make(map[core.UDPConn]net.PacketConn, 8),
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	proxyConn, err := h.client.ListenUDP(nil)
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = proxyConn
	h.Unlock()
	go h.processDownstreamUDP(conn, proxyConn)
	return nil
}

func (h *udpHandler) processDownstreamUDP(conn core.UDPConn, proxyConn net.PacketConn) {
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
		return fmt.Errorf("connection %v->%v does not exists", conn.LocalAddr(), addr)
	}
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
