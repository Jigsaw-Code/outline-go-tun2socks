GOMOBILE=gomobile
GOBIND=$(GOMOBILE) bind
XGOCMD=xgo
BUILDDIR=$(shell pwd)/build
IMPORT_PATH=github.com/Jigsaw-Code/outline-go-tun2socks
ELECTRON_PATH=$(IMPORT_PATH)/outline/electron
LDFLAGS='-s -w'
ANDROID_LDFLAGS='-w' # Don't strip Android debug symbols so we can upload them to crash reporting tools.
TUN2SOCKS_VERSION=v1.16.7
XGO_LDFLAGS='-s -w -X main.version=$(TUN2SOCKS_VERSION)'

ANDROID_BUILDDIR=$(BUILDDIR)/android
ANDROID_ARTIFACT=$(ANDROID_BUILDDIR)/tun2socks.aar
IOS_BUILDDIR=$(BUILDDIR)/ios
IOS_ARTIFACT=$(IOS_BUILDDIR)/Tun2socks.framework
MACOS_BUILDDIR=$(BUILDDIR)/macos
MACOS_ARTIFACT=$(MACOS_BUILDDIR)/Tun2socks.framework
WINDOWS_BUILDDIR=$(BUILDDIR)/windows
LINUX_BUILDDIR=$(BUILDDIR)/linux

ANDROID_BUILD_CMD="$(GOBIND) -a -ldflags $(ANDROID_LDFLAGS) -target=android -tags android -work -o $(ANDROID_ARTIFACT)"
ANDROID_OUTLINE_BUILD_CMD="$(ANDROID_BUILD_CMD) $(IMPORT_PATH)/outline/android $(IMPORT_PATH)/outline/shadowsocks"
ANDROID_INTRA_BUILD_CMD="$(ANDROID_BUILD_CMD) $(IMPORT_PATH)/intra $(IMPORT_PATH)/tunnel $(IMPORT_PATH)/tunnel/intra $(IMPORT_PATH)/tunnel/intra/doh $(IMPORT_PATH)/tunnel/intra/split $(IMPORT_PATH)/tunnel/intra/protect"
IOS_BUILD_CMD="$(GOBIND) -a -ldflags $(LDFLAGS) -bundleid org.outline.tun2socks -target=ios/arm,ios/arm64 -tags ios -o $(IOS_ARTIFACT) $(IMPORT_PATH)/outline/apple $(IMPORT_PATH)/outline/shadowsocks"
MACOS_BUILD_CMD="./tools/$(GOBIND) -a -ldflags $(LDFLAGS) -bundleid org.outline.tun2socks -target=ios/amd64 -tags ios -o $(MACOS_ARTIFACT) $(IMPORT_PATH)/outline/apple $(IMPORT_PATH)/outline/shadowsocks"
WINDOWS_BUILD_CMD="$(XGOCMD) -ldflags $(XGO_LDFLAGS) --targets=windows/386 -dest $(WINDOWS_BUILDDIR) $(ELECTRON_PATH)"
LINUX_BUILD_CMD="$(XGOCMD) -ldflags $(XGO_LDFLAGS) --targets=linux/amd64 -dest $(LINUX_BUILDDIR) $(ELECTRON_PATH)"

define build
	mkdir -p $(1)
	eval $(2)
endef

.PHONY: android-outline android-intra ios linux macos windows clean

all: android-outline android-intra ios linux macos windows

android-outline:
	$(call build,$(ANDROID_BUILDDIR),$(ANDROID_OUTLINE_BUILD_CMD))

android-intra:
	$(call build,$(ANDROID_BUILDDIR),$(ANDROID_INTRA_BUILD_CMD))

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
