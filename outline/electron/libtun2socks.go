// Copyright 2023 The Outline Authors
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

// Use the following command line to generate a C header file:
//   go tool cgo -exportheader ./build/libtun2socks.h ./outline/electron/lib.go

package main

/*
#include <stdint.h> // for uint32_t
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"

	oss "github.com/Jigsaw-Code/outline-go-tun2socks/outline/shadowsocks"
)

// Function returns a tuple [status, pErr]. If pErr is nil (means no errors),
// status is one of the following:
//   - 0 (NoError): can connect by TCP and UDP
//   - 4 (UDPConnectivity): can only connect by TCP (UDP failed)
//   - 3 (AuthenticationFailure): wrong server credentials
//   - 5 (Unreachable): server is not reachable at all
//
// Otherwise if pErr is not nil, it means there are unexpected errors (status
// will be 1 (Unexpected)).
//
// The caller must call ReleaseError(pErr) later to make sure Go will garbage
// collect the error object, otherwise memory leak will happen.
//
//export CheckConnectivity
func CheckConnectivity() (status C.uint32_t, pErr unsafe.Pointer) {
	status = oss.Unexpected
	pErr = nil
	return
}

// If pErr points to an existing error object, this function will return pErr
// object to Go's garbage collector. If pErr is nil, we will do nothing.
//
// In either case, the caller should not use pErr object any more.
//
//export ReleaseError
func ReleaseError(pErr unsafe.Pointer) {
	p := (*cgo.Handle)(pErr)
	if p != nil {
		(*p).Delete()
	}
}
