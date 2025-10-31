// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml

import (
	"encoding/base64"

	"github.com/crewjam/saml/samlsp"
	"go.yaml.in/yaml/v4"
)

// Encoder serializes tracked request cookies as base64 encoded YAML.
type Encoder struct{}

// Encode returns an encoded string representing the TrackedRequest.
func (*Encoder) Encode(in samlsp.TrackedRequest) (string, error) {
	res, err := yaml.Marshal(in)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(res), nil
}

// Decode returns a Tracked request from an encoded string.
func (*Encoder) Decode(in string) (*samlsp.TrackedRequest, error) {
	var req samlsp.TrackedRequest

	rawYAML, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(rawYAML, &req); err != nil {
		return nil, err
	}

	return &req, nil
}
