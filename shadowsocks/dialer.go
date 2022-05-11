package shadowsocks

import (
	"net"

	"github.com/Jigsaw-Code/outline-go-tun2socks/core"
	shadowsocks "github.com/Jigsaw-Code/outline-ss-server/client"
)

type Dialer struct {
	Client shadowsocks.Client
}

func (d Dialer) DialTCP(addr *net.TCPAddr) (core.DoubleStream, error) {
	return d.Client.DialTCP(nil, addr.String())
}

func (d Dialer) ListenUDP(_ core.ReverseUDPConn) (net.PacketConn, error) {
	return d.Client.ListenUDP(nil)
}
