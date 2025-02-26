// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jointoken implements siderolink jointoken parser.
package jointoken

import (
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NodeUniqueToken represents a join token generated for a specific node.
type NodeUniqueToken struct {
	Fingerprint string
	Token       string
}

// NewNodeUniqueToken creates the node unique token.
func NewNodeUniqueToken(fingerprint, token string) *NodeUniqueToken {
	return &NodeUniqueToken{
		Fingerprint: fingerprint,
		Token:       token,
	}
}

// Encode the node unique token to bytes representation.
func (t *NodeUniqueToken) Encode() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Equal is true when the token part is equal.
func (t *NodeUniqueToken) Equal(other *NodeUniqueToken) bool {
	if t == nil || other == nil {
		return false
	}

	return t.Token == other.Token
}

// IsSameFingerprint checks if the tokens have the same fingerprint.
func (t *NodeUniqueToken) IsSameFingerprint(other *NodeUniqueToken) bool {
	if t == nil || other == nil {
		return false
	}

	if t.Fingerprint == "" || other.Fingerprint == "" {
		return false
	}

	return t.Fingerprint == other.Fingerprint
}

// HasToken is true when the token field isn't empty.
func (t *NodeUniqueToken) HasToken() bool {
	if t == nil {
		return false
	}

	return t.Token != ""
}

// ParseNodeUniqueToken from the marshaled version.
func ParseNodeUniqueToken(data string) (*NodeUniqueToken, error) {
	if data == "" {
		return nil, nil //nolint:nilnil
	}

	var t NodeUniqueToken

	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	return &t, nil
}
