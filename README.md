# outline-go-tun2socks

Go package for building [go-tun2socks](https://github.com/eycorsican/go-tun2socks) libraries for macOS, iOS, and Android. Builds go-tun2socks binaries for Linux and Windows.

## Prerequisites

- macOS host (iOS, macOS)
- Xcode (iOS, macOS)
- make
- Go >= 1.13
- A C compiler (e.g.: clang, gcc)
- [gomobile](https://github.com/golang/go/wiki/Mobile) (iOS, macOS, Android)
- [xgo](https://github.com/karalabe/xgo) (Windows, Linux)
- Docker (Windows, Linux)
- Other common utilities (e.g.: git)

Additionally, github.com/Jigsaw-Code/outline-ss-server must be in $GOROOT/src, as well as all of its dependencies.
This is necessary because gomobile does not support modules.  You can fetch these dependencies in the required way by running

```bash
git clone git@github.com:Jigsaw-Code/outline-ss-server.git $GOPATH/src/github.com/Jigsaw-Code/outline-ss-server
GO111MODULE=off go get -d $GOPATH/src/github.com/Jigsaw-Code/outline-ss-server/...
```

## macOS Framework

As of Go 1.13, gomobile does not support building frameworks for macOS. We have patched gomobile to enable building a framework for macOS by replacing the default iOS simulator build.

Until we upstream the change, the (Darwin) binary to enable this behavior is located at `tools/gomobile` and is used by the `build_macos.sh` build script.


## Linux & Windows

We build binaries for Linux and Windows from source without any custom integrations. `xgo` and Docker are required to support cross-compilation.

## Build
```bash
go get -d ./...
./build_[ios|android|macos|windows].sh
```
