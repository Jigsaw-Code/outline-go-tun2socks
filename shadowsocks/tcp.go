package shadowsocks

import (
	"net"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	dialer onet.StreamDialer
}

// NewTCPHandler returns a Shadowsocks TCP connection handler.
func NewTCPHandler(client onet.StreamDialer) core.TCPConnHandler {
	return &tcpHandler{client}
}

func (h *tcpHandler) Handle(conn net.Conn, target *net.TCPAddr) error {
	proxyConn, err := h.dialer.Dial(target.String())
	if err != nil {
		return err
	}
	// TODO: Request upstream to make `conn` a `core.TCPConn` so we can avoid this type assertion.
	go onet.Relay(conn.(core.TCPConn), proxyConn)
	return nil
}
