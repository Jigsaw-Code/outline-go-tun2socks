package tunnel

import (
	"errors"
	"io"
	"os"

	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Import simple log for the side effect of making logs printable.
	"golang.org/x/sys/unix"
)

const vpnMtu = 1500

// MakeTunFile returns an os.File object from a TUN file descriptor `fd`
// without taking ownership of the file descriptor.
func MakeTunFile(fd int) (*os.File, error) {
	if fd < 0 {
		return nil, errors.New("Must provide a valid TUN file descriptor")
	}
	// Make a copy of `fd` so that os.File's finalizer doesn't close `fd`.
	newfd, err := unix.Dup(fd)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(newfd), "")
	if file == nil {
		return nil, errors.New("Failed to open TUN file descriptor")
	}
	return file, nil
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
