// Copyright 2019 The Outline Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/outline/shadowsocks"
	"github.com/Jigsaw-Code/outline-go-tun2socks/shadowsocks"
	"github.com/Jigsaw-Code/outline-ss-server/client"
	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Register a simple logger.
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/dnsfallback"
	"github.com/eycorsican/go-tun2socks/tun"
)

const (
	mtu        = 1500
	udpTimeout = 30 * time.Second
	persistTun = true // Linux: persist the TUN interface after the last open file descriptor is closed.
)

var args struct {
	tunAddr           *string
	tunGw             *string
	tunMask           *string
	tunName           *string
	tunDNS            *string
	proxyHost         *string
	proxyPort         *int
	proxyPassword     *string
	proxyCipher       *string
	proxyPrefix       *string
	logLevel          *string
	checkConnectivity *bool
	dnsFallback       *bool
	version           *bool
}
var version string // Populated at build time through `-X main.version=...`
var lwipWriter io.Writer

func main() {
	args.tunAddr = flag.String("tunAddr", "10.0.85.2", "TUN interface IP address")
	args.tunGw = flag.String("tunGw", "10.0.85.1", "TUN interface gateway")
	args.tunMask = flag.String("tunMask", "255.255.255.0", "TUN interface network mask; prefixlen for IPv6")
	args.tunDNS = flag.String("tunDNS", "1.1.1.1,9.9.9.9,208.67.222.222", "Comma-separated list of DNS resolvers for the TUN interface (Windows only)")
	args.tunName = flag.String("tunName", "tun0", "TUN interface name")
	args.proxyHost = flag.String("proxyHost", "", "Shadowsocks proxy hostname or IP address")
	args.proxyPort = flag.Int("proxyPort", 0, "Shadowsocks proxy port number")
	args.proxyPassword = flag.String("proxyPassword", "", "Shadowsocks proxy password")
	args.proxyCipher = flag.String("proxyCipher", "chacha20-ietf-poly1305", "Shadowsocks proxy encryption cipher")
	args.proxyPrefix = flag.String("proxyPrefix", "", "Shadowsocks connection prefix, URI-encoded (unsafe)")
	args.logLevel = flag.String("logLevel", "info", "Logging level: debug|info|warn|error|none")
	args.dnsFallback = flag.Bool("dnsFallback", false, "Enable DNS fallback over TCP (overrides the UDP handler).")
	args.checkConnectivity = flag.Bool("checkConnectivity", false, "Check the proxy TCP and UDP connectivity and exit.")
	args.version = flag.Bool("version", false, "Print the version and exit.")

	flag.Parse()

	if *args.version {
		fmt.Println(version)
		os.Exit(0)
	}

	setLogLevel(*args.logLevel)

	// Validate proxy flags
	if *args.proxyHost == "" {
		log.Errorf("Must provide a Shadowsocks proxy host name or IP address")
		os.Exit(oss.IllegalConfiguration)
	} else if *args.proxyPort <= 0 || *args.proxyPort > 65535 {
		log.Errorf("Must provide a valid Shadowsocks proxy port [1:65535]")
		os.Exit(oss.IllegalConfiguration)
	} else if *args.proxyPassword == "" {
		log.Errorf("Must provide a Shadowsocks proxy password")
		os.Exit(oss.IllegalConfiguration)
	} else if *args.proxyCipher == "" {
		log.Errorf("Must provide a Shadowsocks proxy encryption cipher")
		os.Exit(oss.IllegalConfiguration)
	}

	if *args.checkConnectivity {
		connErrCode, err := oss.CheckConnectivity(*args.proxyHost, *args.proxyPort, *args.proxyPassword, *args.proxyCipher)
		log.Debugf("Connectivity checks error code: %v", connErrCode)
		if err != nil {
			log.Errorf("Failed to perform connectivity checks: %v", err)
		}
		os.Exit(connErrCode)
	}

	// Open TUN device
	dnsResolvers := strings.Split(*args.tunDNS, ",")
	tunDevice, err := tun.OpenTunDevice(*args.tunName, *args.tunAddr, *args.tunGw, *args.tunMask, dnsResolvers, persistTun)
	if err != nil {
		log.Errorf("Failed to open TUN device: %v", err)
		os.Exit(oss.SystemMisconfigured)
	}
	// Output packets to TUN device
	core.RegisterOutputFn(tunDevice.Write)

	ssclient, err := client.NewClient(*args.proxyHost, *args.proxyPort, *args.proxyPassword, *args.proxyCipher)
	if err != nil {
		log.Errorf("Failed to construct Shadowsocks client: %v", err)
		os.Exit(oss.IllegalConfiguration)
	}
	prefixBytes, err := url.PathUnescape(*args.proxyPrefix)
	if err != nil {
		log.Errorf("\"%s\" could not be URI-decoded", *args.proxyPrefix)
		os.Exit(oss.IllegalConfiguration)
	}
	ssclient.SetTCPSaltGenerator(client.NewPrefixSaltGenerator([]byte(prefixBytes)))
	// Register TCP and UDP connection handlers
	core.RegisterTCPConnHandler(shadowsocks.NewTCPHandler(ssclient))
	if *args.dnsFallback {
		// UDP connectivity not supported, fall back to DNS over TCP.
		log.Debugf("Registering DNS fallback UDP handler")
		core.RegisterUDPConnHandler(dnsfallback.NewUDPHandler())
	} else {
		core.RegisterUDPConnHandler(shadowsocks.NewUDPHandler(ssclient, udpTimeout))
	}

	// Configure LWIP stack to receive input data from the TUN device
	lwipWriter := core.NewLWIPStack()
	go func() {
		_, err := io.CopyBuffer(lwipWriter, tunDevice, make([]byte, mtu))
		if err != nil {
			log.Errorf("Failed to write data to network stack: %v", err)
			os.Exit(oss.Unexpected)
		}
	}()

	log.Infof("tun2socks running...")

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	sig := <-osSignals
	log.Debugf("Received signal: %v", sig)
}

func setLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DEBUG)
	case "info":
		log.SetLevel(log.INFO)
	case "warn":
		log.SetLevel(log.WARN)
	case "error":
		log.SetLevel(log.ERROR)
	case "none":
		log.SetLevel(log.NONE)
	default:
		log.SetLevel(log.INFO)
	}
}
