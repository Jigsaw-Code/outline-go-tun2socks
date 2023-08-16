// Copyright 2022 The Outline Authors
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

package sdk

import (
	"fmt"

	"github.com/Jigsaw-Code/outline-sdk/network"
	"github.com/Jigsaw-Code/outline-sdk/network/lwip2transport"
	"github.com/Jigsaw-Code/outline-sdk/transport"
)

// TODO: move to "outline-apps/internal" once we have migrated to monorepo

const (
	connectivityTestDNSResolver  = "1.1.1.1:53"
	connectivityTestTargetDomain = "www.google.com"
)

type OutlineClientDevice struct {
	t2s network.IPDevice
	pp  *outlinePacketProxy
	sd  transport.StreamDialer
}

func NewOutlineClientDevice(configJSON string) (d *OutlineClientDevice, err error) {
	d = &OutlineClientDevice{}

	d.sd, err = NewOutlineStreamDialer(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP dialer: %w", err)
	}

	d.pp, err = newOutlinePacketProxy(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP proxy: %w", err)
	}

	d.t2s, err = lwip2transport.ConfigureDevice(d.sd, d.pp)
	if err != nil {
		return nil, fmt.Errorf("failed to configure lwIP: %w", err)
	}

	return
}

func (d *OutlineClientDevice) Close() error {
	return d.t2s.Close()
}

func (d *OutlineClientDevice) Refresh() error {
	return d.pp.testConnectivityAndRefresh(connectivityTestDNSResolver, connectivityTestTargetDomain)
}
