//go:build tools
// +build tools

// See https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// and https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md

package tools

import (
	_ "golang.org/x/mobile/cmd/gomobile"
	_ "src.techknowlogick.com/xgo"
)
