BUILDDIR=$(CURDIR)/build
GOBIN=$(CURDIR)/bin

GOMOBILE=$(GOBIN)/gomobile
# Add GOBIN to $PATH so `gomobile` can find `gobind`.
GOBIND=env PATH="$(GOBIN):$(PATH)" "$(GOMOBILE)" bind
IMPORT_HOST=github.com
IMPORT_PATH=$(IMPORT_HOST)/Jigsaw-Code/outline-go-tun2socks

.PHONY: android apple linux windows intra clean clean-all

all: intra android linux apple windows

# Don't strip Android debug symbols so we can upload them to crash reporting tools.
ANDROID_BUILD_CMD=$(GOBIND) -a -ldflags '-w' -target=android -tags android -work

intra: $(BUILDDIR)/intra/tun2socks.aar

$(BUILDDIR)/intra/tun2socks.aar: $(GOMOBILE)
	mkdir -p "$(BUILDDIR)/intra"
	$(ANDROID_BUILD_CMD) -o "$@" $(IMPORT_PATH)/intra $(IMPORT_PATH)/intra/android $(IMPORT_PATH)/intra/doh $(IMPORT_PATH)/intra/split $(IMPORT_PATH)/intra/protect

android: $(BUILDDIR)/android/tun2socks.aar

$(BUILDDIR)/android/tun2socks.aar: $(GOMOBILE)
	mkdir -p "$(BUILDDIR)/android"
	$(ANDROID_BUILD_CMD) -o "$@" $(IMPORT_PATH)/outline/tun2socks $(IMPORT_PATH)/outline/shadowsocks

# TODO(fortuna): -s strips symbols and is obsolete. Why are we using it?
$(BUILDDIR)/ios/Tun2socks.xcframework: $(GOMOBILE)
  # -iosversion should match what outline-client supports.
	$(GOBIND) -iosversion=11.0 -target=ios,iossimulator -o $@ -ldflags '-s -w' -bundleid org.outline.tun2socks $(IMPORT_PATH)/outline/tun2socks $(IMPORT_PATH)/outline/shadowsocks

$(BUILDDIR)/macos/Tun2socks.xcframework: $(GOMOBILE)
  # MACOSX_DEPLOYMENT_TARGET and -iosversion should match what outline-client supports.
	export MACOSX_DEPLOYMENT_TARGET=10.14; $(GOBIND) -iosversion=13.1 -target=macos,maccatalyst -o $@ -ldflags '-s -w' -bundleid org.outline.tun2socks $(IMPORT_PATH)/outline/tun2socks $(IMPORT_PATH)/outline/shadowsocks

apple: $(BUILDDIR)/apple/Tun2socks.xcframework

$(BUILDDIR)/apple/Tun2socks.xcframework: $(BUILDDIR)/ios/Tun2socks.xcframework $(BUILDDIR)/macos/Tun2socks.xcframework
	find $^ -name "Tun2socks.framework" -type d | xargs -I {} echo " -framework {} " | \
		xargs xcrun xcodebuild -create-xcframework -output "$@"

XGO=$(GOBIN)/xgo
TUN2SOCKS_VERSION=v1.16.11
XGO_LDFLAGS='-s -w -X main.version=$(TUN2SOCKS_VERSION)'
ELECTRON_PKG=outline/electron


LINUX_BUILDDIR=$(BUILDDIR)/linux

linux: $(LINUX_BUILDDIR)/tun2socks

$(LINUX_BUILDDIR)/tun2socks: $(XGO)
	mkdir -p "$(LINUX_BUILDDIR)/$(IMPORT_PATH)"
	$(XGO) -ldflags $(XGO_LDFLAGS) --targets=linux/amd64 -dest "$(LINUX_BUILDDIR)" -pkg $(ELECTRON_PKG) .
	mv "$(LINUX_BUILDDIR)/$(IMPORT_PATH)-linux-amd64" "$@"
	rm -r "$(LINUX_BUILDDIR)/$(IMPORT_HOST)"


WINDOWS_BUILDDIR=$(BUILDDIR)/windows

windows: $(WINDOWS_BUILDDIR)/tun2socks.exe

$(WINDOWS_BUILDDIR)/tun2socks.exe: $(XGO)
	mkdir -p "$(WINDOWS_BUILDDIR)/$(IMPORT_PATH)"
	$(XGO) -ldflags $(XGO_LDFLAGS) --targets=windows/386 -dest "$(WINDOWS_BUILDDIR)" -pkg $(ELECTRON_PKG) .
	mv "$(WINDOWS_BUILDDIR)/$(IMPORT_PATH)-windows-386.exe" "$@"
	rm -r "$(WINDOWS_BUILDDIR)/$(IMPORT_HOST)"


$(GOMOBILE): go.mod
	env GOBIN="$(GOBIN)" go install golang.org/x/mobile/cmd/gomobile
	env GOBIN="$(GOBIN)" $(GOMOBILE) init

$(XGO): go.mod
	env GOBIN="$(GOBIN)" go install github.com/crazy-max/xgo

go.mod: tools.go
	go mod tidy
	touch go.mod

clean:
	rm -rf "$(BUILDDIR)"
	go clean

clean-all: clean
	rm -rf "$(GOBIN)"
