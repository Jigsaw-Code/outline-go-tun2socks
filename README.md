# outline-go-tun2socks

Go package for building [go-tun2socks](https://github.com/eycorsican/go-tun2socks)-based clients for [Outline](https://getoutline.org) and [Intra](https://getintra.org) (now with support for [Choir](https://github.com/Jigsaw-Code/choir) metrics).  For macOS, iOS, and Android, the output is a library; for Linux and Windows it is a command-line executable.

## Prerequisites

- macOS host (iOS, macOS)
- Xcode (iOS, macOS)
- make
- Go >= 1.14
- A C compiler (e.g.: clang, gcc)
- [gomobile](https://github.com/golang/go/wiki/Mobile) (iOS, macOS, Android)
- [xgo](https://github.com/techknowlogick/xgo) (Windows, Linux)
- Docker (Windows, Linux)
- Other common utilities (e.g.: git)


## Linux & Windows

We build binaries for Linux and Windows from source without any custom integrations. `xgo` and Docker are required to support cross-compilation.

## Build
```bash
export PATH=$PATH:"$(go env GOPATH)/bin"
go mod tidy
./build_[ios|android|macos|windows].sh
```
