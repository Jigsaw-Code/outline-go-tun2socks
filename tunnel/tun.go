package tunnel

import (
	"io"

	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Import simple log for the side effect of making logs printable.
	"golang.org/x/sys/unix"
)

const vpnMtu = 1500

// TUNFile is an io.ReadWriter wrapping a TUN file descriptor (UNIX platforms only).
// This is a substitute for os.NewFile that avoids taking ownership of the file.
type TUNFile int

func (f TUNFile) Read(buf []byte) (int, error) {
	return unix.Read(int(f), buf)
}

func (f TUNFile) Write(buf []byte) (int, error) {
	return unix.Write(int(f), buf)
}

// ProcessInputPackets reads packets from a TUN device `tun` and writes them to `tunnel`.
func ProcessInputPackets(tunnel Tunnel, tun io.Reader, onError func(error)) {
	buffer := make([]byte, vpnMtu)
	for tunnel.IsConnected() {
		len, err := tun.Read(buffer)
		if err != nil {
			log.Warnf("Failed to read packet from TUN: %v", err)
			if onError != nil {
				onError(err)
			}
			continue
		}
		if len == 0 {
			log.Infof("Read EOF from TUN")
			continue
		}
		_, err = tunnel.Write(buffer)
		if err != nil && onError != nil {
			onError(err)
		}
	}
}
