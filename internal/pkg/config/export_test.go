// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import "bytes"

// FromBytes loads the config from bytes.
func FromBytes(data []byte) (*Params, error) {
	return parseConfig(bytes.NewBuffer(data))
}
