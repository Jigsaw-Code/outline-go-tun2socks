package shadowsocks

import (
	"net"

	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/eycorsican/go-tun2socks/core"
)

type TCPConnHandler interface {
	core.TCPConnHandler

	// SetTCPPrefix replaces the beginning of each outgoing connection with
	// the specified bytes.  Note: This reduces the IV entropy, and can easily
	// break the security of Shadowsocks.
	// This method is not thread-safe, so it must only be called before the
	// handler is registered.
	SetTCPPrefix(prefix []byte)
}

type tcpHandler struct {
	client shadowsocks.Client
}

// NewTCPHandler returns a Shadowsocks TCP connection handler.
//
// `host` is the hostname of the Shadowsocks proxy server.
// `port` is the port of the Shadowsocks proxy server.
// `password` is password used to authenticate to the server.
// `cipher` is the encryption cipher of the Shadowsocks proxy.
func NewTCPHandler(host string, port int, password, cipher string) TCPConnHandler {
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

func (h *tcpHandler) SetTCPPrefix(prefix []byte) {
	h.client.SetTCPSaltGenerator(shadowsocks.NewPrefixSaltGenerator(prefix))
}
