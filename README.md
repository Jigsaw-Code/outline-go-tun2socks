# outline-go-tun2socks

Go package for building [go-tun2socks](https://github.com/eycorsican/go-tun2socks) libraries for macOS, iOS, and Android. Builds go-tun2socks binaries for Linux and Windows.

## Prerequisites

- macOS host (iOS, macOS)
- Xcode (iOS, macOS)
- make
- Go >= 1.12
- A C compiler (e.g.: clang, gcc)
- [gomobile](https://github.com/golang/go/wiki/Mobile) (iOS, macOS, Android)
- [xgo](https://github.com/karalabe/xgo) (Windows, Linux)
- Docker (Windows, Linux)
- Other common utilities (e.g.: git)

## Apple Golang Runtime

We use a custom Golang runtime to build the iOS and macOS framework built off Go 1.12. The [patch](https://go-review.googlesource.com/c/go/+/159117) improves memory reporting to the OS. This should not be necessary after Go 1.13 is released (scheduled for August 2019).

You can use our pre-compiled Darwin binary, at `tools/go` or build it yourself:

```bash
# We assume that Go is installed in /usr/local; this may vary on your system.
cd /usr/local/
# Temporarily move the current Go version, so as not to clobber $PATH.
mv go go1.12
# Download the Go source.
git clone https://go.googlesource.com/go
cd go
git checkout release-branch.go1.12
# Apply the patch.
git fetch https://go.googlesource.com/go refs/changes/17/159117/5 && git cherry-pick FETCH_HEAD
# Update the version.
echo "go1.12-dev-runtime" > VERSION
# Build the runtime.
cd src
GOROOT_BOOTSTRAP=/usr/local/go1.12/ ./make.bash
# Verify that the installed binary matches the custom version (i.e. 'go version go1.12-dev-runtime darwin/amd64').
go version
```

After building the framework, you can delete the custom runtime and revert to Go 1.12.

## macOS Framework

As of Go 1.12, gomobile does not support building frameworks for macOS. We have patched gomobile to enable building a framework for macOS by replacing the default iOS simulator build.

Until we upstream the change, the (Darwin) binary to enable this behavior is located at `tools/gomobile`.

```bash
    # Find out the path of the installed gomobile (i.e. ~/go/bin/gomobile).
    which gomobile
    # Temporarily rename the installed gomobile.
    mv ~/go/bin/gomobile ~/go/bin/gomobile-prod
    # Copy the patched gomobile binary.
    ln -s `pwd`/tools/gomobile ~/go/bin/gomobile
    # Initialize gomobile.
    gomobile init
    # Build the macOS framework.
    ./build_macos
    # Revert the changes.
    rm ~/go/bin/gomobile
    mv ~/go/bin/gomobile-prod ~/go/bin/gomobile
```

## Linux & Windows

We build binaries for Linux and Windows from source without any custom integrations. `xgo` and Docker are required to support cross-compilation.

## Build
```bash
go get -d ./...
./build_[ios|android|macos|windows].sh
```
