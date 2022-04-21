BUILDDIR=$(CURDIR)/build
GOBIN=$(CURDIR)/bin

GOMOBILE=$(GOBIN)/gomobile
IMPORT_PATH=github.com/Jigsaw-Code/outline-go-tun2socks

.PHONY: android-outline android-intra linux apple windows clean

all: android-outline android-intra linux apple windows

ANDROID_LDFLAGS='-w' # Don't strip Android debug symbols so we can upload them to crash reporting tools.
ANDROID_BUILDDIR=$(BUILDDIR)/android
ANDROID_ARTIFACT=$(ANDROID_BUILDDIR)/tun2socks.aar
ANDROID_BUILD_CMD=$(GOMOBILE) bind -a -ldflags $(ANDROID_LDFLAGS) -target=android -tags android -work -o $(ANDROID_ARTIFACT)

android-outline: $(GOMOBILE)
	$(ANDROID_BUILD_CMD) $(IMPORT_PATH)/outline/android $(IMPORT_PATH)/outline/shadowsocks

android-intra: $(GOMOBILE)
	$(ANDROID_BUILD_CMD) $(IMPORT_PATH)/intra $(IMPORT_PATH)/intra/android $(IMPORT_PATH)/intra/doh $(IMPORT_PATH)/intra/split $(IMPORT_PATH)/intra/protect


apple: $(BUILDDIR)/apple/Tun2socks.xcframework

$(BUILDDIR)/apple/Tun2socks.xcframework: $(GOMOBILE)
  # TODO(fortuna): -s strips symbols and is obsolete. Why are we using it?
	$(GOMOBILE) bind -a -ldflags '-s -w' -bundleid org.outline.tun2socks -target=ios,iossimulator,macos,maccatalyst -o "$@" $(IMPORT_PATH)/outline/apple $(IMPORT_PATH)/outline/shadowsocks


XGO=$(GOBIN)/xgo
TUN2SOCKS_VERSION=v1.16.11
XGO_LDFLAGS='-s -w -X main.version=$(TUN2SOCKS_VERSION)'
ELECTRON_PATH=$(IMPORT_PATH)/outline/electron


LINUX_BUILDDIR=$(BUILDDIR)/linux

linux: $(LINUX_BUILDDIR)/tun2socks

$(LINUX_BUILDDIR)/tun2socks: $(XGO)
	$(XGO) -ldflags $(XGO_LDFLAGS) --targets=linux/amd64 -dest $(LINUX_BUILDDIR) $(ELECTRON_PATH)
	mv $(LINUX_BUILDDIR)/electron-linux-amd64 "$@"


WINDOWS_BUILDDIR=$(BUILDDIR)/windows

windows: $(WINDOWS_BUILDDIR)/tun2socks.exe

$(WINDOWS_BUILDDIR)/tun2socks.exe: $(XGO)
	$(XGO) -ldflags $(XGO_LDFLAGS) --targets=windows/386 -dest $(WINDOWS_BUILDDIR) $(ELECTRON_PATH)
	mv $(WINDOWS_BUILDDIR)/electron-windows-4.0-386.exe "$@"


$(GOMOBILE): go.mod
	GOBIN=$(GOBIN) go install golang.org/x/mobile/cmd/gomobile
	$(GOMOBILE) init

$(XGO): go.mod
	GOBIN=$(GOBIN) go install src.techknowlogick.com/xgo

go.mod: tools.go
	go mod tidy
	touch go.mod

clean:
	rm -rf $(BUILDDIR) $(GOBIN)
