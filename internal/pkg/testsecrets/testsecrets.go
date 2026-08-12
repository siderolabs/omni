// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package testsecrets provides Talos secrets bundles for tests.
//
// Generating a bundle costs around a second of CPU because of the RSA service account key.
// Because of this, bundles are generated once per version contract per test process and shared.
// Tests do not need unique secrets.
package testsecrets

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

var (
	cacheMu sync.Mutex
	cache   = map[string][]byte{}
)

// Bundle returns a secrets bundle for the given version contract, which may be nil.
func Bundle(vc *config.VersionContract) (*secrets.Bundle, error) {
	data, err := BundleData(vc)
	if err != nil {
		return nil, err
	}

	bundle := secrets.Bundle{Clock: secrets.NewClock()}

	if err = json.Unmarshal(data, &bundle); err != nil {
		return nil, err
	}

	return &bundle, nil
}

// BundleData returns a marshaled secrets bundle for the given version contract, which may be nil.
func BundleData(vc *config.VersionContract) ([]byte, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if data, ok := cache[vc.String()]; ok {
		return bytes.Clone(data), nil
	}

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), vc)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	cache[vc.String()] = data

	return bytes.Clone(data), nil
}
