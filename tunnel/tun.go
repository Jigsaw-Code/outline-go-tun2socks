package tunnel

import (
	"errors"
	"os"

	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Import simple log for the side effect of making logs printable.
)

const vpnMtu = 1500

// MakeTunFile returns an os.File object from a TUN file descriptor `fd`.
func MakeTunFile(fd int) (*os.File, error) {
	if fd < 0 {
		return nil, errors.New("Must provide a valid TUN file descriptor")
	}
	file := os.NewFile(uintptr(fd), "")
	if file == nil {
		return nil, errors.New("Failed to open TUN file descriptor")
	}
	return file, nil
}

// ProcessInputPackets reads packets from a TUN device `tun` and writes them to `tunnel`.
func ProcessInputPackets(tunnel Tunnel, tun *os.File) {
	buffer := make([]byte, vpnMtu)
	for tunnel.IsConnected() {
		len, err := tun.Read(buffer)
		if err != nil {
			log.Warnf("Failed to read packet from TUN: %v", err)
			continue
		}
		if len == 0 {
			log.Infof("Read EOF from TUN")
			continue
		}
		tunnel.Write(buffer)
	}
}
