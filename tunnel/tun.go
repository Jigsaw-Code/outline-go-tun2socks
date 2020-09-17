package tunnel

import (
	"errors"
	"os"

	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Import simple log for the side effect of making logs printable.
	"golang.org/x/sys/unix"
)

const vpnMtu = 1500

// MakeTunFile returns an os.File object from a TUN file descriptor `fd`.
// The returned os.File holds a separate reference to the underlying file,
// so the file will not be closed until both `fd` and the os.File are
// separately closed.  (UNIX only.)
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
