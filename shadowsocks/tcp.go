package shadowsocks

import (
	"net"

	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	client shadowsocks.Client
}

// NewTCPHandler returns a Shadowsocks TCP connection handler.
func NewTCPHandler(client shadowsocks.Client) core.TCPConnHandler {
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
