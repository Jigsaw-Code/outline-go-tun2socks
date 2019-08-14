GOCMD=go
GOMOBILE=gomobile
GOBIND=$(GOMOBILE) bind
GOBUILD=$(GOCMD) build
XGOCMD=xgo
BUILDDIR=$(shell pwd)/build
IMPORT_PATH=github.com/Jigsaw-Code/outline-go-tun2socks
LDFLAGS='-s -w'
ANDROID_LDFLAGS='-w' # Don't strip Android debug symbols so we can upload them to crash reporting tools.
TUN2SOCKS_VERSION=v1.16.2
TUN2SOCKS_SRC_PATH=$(GOPATH)/src/github.com/eycorsican/go-tun2socks
TUN2SOCKS_MOD_PATH=$(GOPATH)/pkg/mod/github.com/eycorsican/go-tun2socks\@$(TUN2SOCKS_VERSION)
XGO_LDFLAGS='-s -w -X main.version=$(TUN2SOCKS_VERSION)'
XGO_BUILD_TAGS='dnscache dnsfallback socks'

ANDROID_BUILDDIR=$(BUILDDIR)/android
ANDROID_ARTIFACT=$(ANDROID_BUILDDIR)/tun2socks.aar
IOS_BUILDDIR=$(BUILDDIR)/ios
IOS_ARTIFACT=$(IOS_BUILDDIR)/Tun2socks.framework
MACOS_BUILDDIR=$(BUILDDIR)/macos
MACOS_ARTIFACT=$(MACOS_BUILDDIR)/Tun2socks.framework
WINDOWS_BUILDDIR=$(BUILDDIR)/windows
LINUX_BUILDDIR=$(BUILDDIR)/linux

ANDROID_BUILD_CMD="GO111MODULE=off $(GOBIND) -a -ldflags $(ANDROID_LDFLAGS) -target=android -tags android -work -o $(ANDROID_ARTIFACT) $(IMPORT_PATH)/android $(IMPORT_PATH)/tunnel $(IMPORT_PATH)/tunnel/intra"
IOS_BUILD_CMD="GO111MODULE=off $(GOBIND) -a -ldflags $(LDFLAGS) -bundleid org.outline.tun2socks -target=ios/arm,ios/arm64 -tags ios -o $(IOS_ARTIFACT) $(IMPORT_PATH)/apple"
MACOS_BUILD_CMD="GO111MODULE=off $(GOBIND) -a -ldflags $(LDFLAGS) -bundleid org.outline.tun2socks -target=ios/amd64 -tags ios -o $(MACOS_ARTIFACT) $(IMPORT_PATH)/apple"
WINDOWS_BUILD_CMD="$(XGOCMD) -ldflags $(XGO_LDFLAGS) -tags $(XGO_BUILD_TAGS)  --targets=windows/386 -dest $(WINDOWS_BUILDDIR) $(TUN2SOCKS_SRC_PATH)/cmd/tun2socks"
LINUX_BUILD_CMD="$(XGOCMD) -ldflags $(XGO_LDFLAGS) -tags $(XGO_BUILD_TAGS) --targets=linux/amd64 -dest $(LINUX_BUILDDIR) $(TUN2SOCKS_SRC_PATH)/cmd/tun2socks"

define build
	$(call modularize)
	mkdir -p $(1)
	eval $(2)
	$(call undo_modularize)
endef

# Workaround to modularize go-tun2socks and gomobile.
define modularize
	# gomobile does not yet support modules.
	# Symlink the current module and the go-tun2socks module in $GOPATH.
	# go-tun2socks should not be in $GOPATH for this to work.
	# TODO: remove this once gomobile enables modules in Go 1.13.
	mkdir -p $(GOPATH)/src/$(shell dirname $(IMPORT_PATH))
	ln -s $(shell pwd) $(GOPATH)/src/$(IMPORT_PATH)
	mkdir -p $(shell dirname $(TUN2SOCKS_SRC_PATH))
	ln -s $(TUN2SOCKS_MOD_PATH) $(TUN2SOCKS_SRC_PATH)
endef

define undo_modularize
	rm $(GOPATH)/src/$(IMPORT_PATH) || true
	rm $(TUN2SOCKS_SRC_PATH) || true
endef

.PHONY: android ios linux macos windows clean

all: android ios linux macos windows

android:
	$(call build,$(ANDROID_BUILDDIR),$(ANDROID_BUILD_CMD))

ios:
	$(call build,$(IOS_BUILDDIR),$(IOS_BUILD_CMD))

linux:
	$(call build,$(LINUX_BUILDDIR),$(LINUX_BUILD_CMD))

macos:
	$(call build,$(MACOS_BUILDDIR),$(MACOS_BUILD_CMD))

windows:
	$(call build,$(WINDOWS_BUILDDIR),$(WINDOWS_BUILD_CMD))

clean:
	rm -rf $(BUILDDIR)
	$(call undo_modularize)
