package shadowsocks

import (
	"net"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	client shadowsocks.Client
}

// NewTCPHandler TODO
func NewTCPHandler(host string, port int, password, cipher string) core.TCPConnHandler {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return nil
	}
	return &tcpHandler{client}
}

func (h *tcpHandler) Handle(conn net.Conn, target *net.TCPAddr) error {
	proxyConn, err := h.client.DialTCP(nil, target.String())
	if err != nil {
		return err
	}
	// TODO: Request upstream to make `conn` a `core.TCPConn` so we can avoid this type assertion.
	go onet.Relay(conn.(core.TCPConn), proxyConn)
	return nil
}
