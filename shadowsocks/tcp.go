package shadowsocks

import (
	"math/rand"
	"net"
	"time"

	onet "github.com/Jigsaw-Code/outline-ss-server/net"
	"github.com/Jigsaw-Code/outline-ss-server/shadowsocks"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	client shadowsocks.Client
}

// NewTCPHandler returns a Shadowsocks TCP connection handler.
//
// `host` is the hostname of the Shadowsocks proxy server.
// `port` is the port of the Shadowsocks proxy server.
// `password` is password used to authenticate to the server.
// `cipher` is the encryption cipher of the Shadowsocks proxy.
func NewTCPHandler(host string, port int, password, cipher string) core.TCPConnHandler {
	client, err := shadowsocks.NewClient(host, port, password, cipher)
	if err != nil {
		return nil
	}
	return &tcpHandler{client}
}

// This code contains an optimization to send the initial client payload along with
// the Shadowsocks handshake.  This saves one packet during connection, and also
// reduces the distinctiveness of the connection pattern.
//
// Normally, the initial payload will be sent as soon as the socket is connected,
// except for delays due to inter-process communication.  However, some protocols
// expect the server to send data first, in which case there is no client payload.
// We therefore use a short delay, longer than any reasonable IPC but similar to
// typical network latency.  (In an emulator, the 90th percentile delay was ~1 ms.)
// If no client payload is received by this time, we connect without it.
const helloWait = 20 * time.Millisecond

func (h *tcpHandler) relay(clientConn core.TCPConn, proxyConn onet.DuplexConn, target *net.TCPAddr) {
	// Choose a buffer size big enough that the initial payload is likely to fit,
	// small enough to avoid memory pressure, and random enough to avoid distinctive
	// packet sizes if it is filled. In a basic web browsing test, 99% of initial
	// payloads were <670 bytes
	buf := make([]byte, 1024+rand.Intn(512))
	before := time.Now()
	clientConn.SetReadDeadline(before.Add(helloWait))
	n, _ := clientConn.Read(buf)
	clientConn.SetReadDeadline(time.Time{})
	log.Debugf("Got initial %d bytes in %v", n, time.Now().Sub(before))
	destConn, err := h.client.DialDestinationTCP(proxyConn, target.String(), buf[:n])
	if err != nil {
		log.Warnf("Couldn't connect to destination: %v", err)
		clientConn.Close()
		proxyConn.Close()
		return
	}
	onet.Relay(clientConn, destConn)
}

func (h *tcpHandler) Handle(conn net.Conn, target *net.TCPAddr) error {
	proxyConn, err := h.client.DialProxyTCP(nil)
	if err != nil {
		return err // We couldn't reach the proxy server.
	}
	// TODO: Request upstream to make `conn` a `core.TCPConn` so we can avoid this type assertion.
	go h.relay(conn.(core.TCPConn), proxyConn, target)
	return nil
}
