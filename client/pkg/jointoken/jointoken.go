// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jointoken implements siderolink jointoken parser.
package jointoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jxskiss/base62"
)

// JoinTokenLen number of random bytes to be encoded in the join token.
// The real length of the token will depend on the base62 encoding,
// whose lengths happpens to be non-deterministic.
const JoinTokenLen = 32

// Generate the random join token string.
func Generate() (string, error) {
	b := make([]byte, JoinTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	token := base62.EncodeToString(b)

	return token, nil
}

// ExtraData is the type of the extra token data.
type ExtraData map[string]string

func (e ExtraData) signature(token string) ([]byte, error) {
	mac := hmac.New(sha256.New, []byte(token))

	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	if _, err = mac.Write(data); err != nil {
		return nil, err
	}

	return mac.Sum(nil), nil
}

const (
	v1Prefix = "v1:"
	v2Prefix = "v2:"
)

const (
	// VersionPlain is the random token string.
	VersionPlain = "plain"
	// Version1 is the signed token that contains extra data.
	Version1 = "1"
	// Version2 is the same as version 1, but the provider uses individual tokens.
	Version2 = "2"
)

// NewPlain token creates the token without extra data.
func NewPlain(token string) JoinToken {
	return JoinToken{
		token: token,
	}
}

// NewWithExtraData creates the token with extra data.
func NewWithExtraData(token string, extraData map[string]string) (JoinToken, error) {
	t := JoinToken{
		ExtraData: extraData,
		token:     token,
	}

	var err error

	t.Signature, err = t.ExtraData.signature(token)
	if err != nil {
		return t, err
	}

	return t, nil
}

// JoinToken is the siderolink join token.
// Custom type adds methods for encoding/decoding extra data from the token.
type JoinToken struct {
	ExtraData ExtraData `json:"extra_data"`

	token   string
	Version string `json:"-"`

	Signature []byte `json:"signature"`
}

// Parse reads string into token.
func Parse(value string) (JoinToken, error) {
	var res JoinToken

	res.Version = VersionPlain

	var prefix string

	switch {
	case strings.HasPrefix(value, v1Prefix):
		res.Version = Version1

		prefix = v1Prefix
	case strings.HasPrefix(value, v2Prefix):
		res.Version = Version2

		prefix = v2Prefix
	}

	if res.Version != VersionPlain {
		data, err := base64.StdEncoding.DecodeString(strings.TrimLeft(value, prefix))
		if err != nil {
			return res, err
		}

		return res, json.Unmarshal(data, &res)
	}

	res.token = value
	res.Version = VersionPlain

	return res, nil
}

// IsValid checks the signature or plain token.
func (t JoinToken) IsValid(token string) bool {
	if t.Signature == nil {
		return t.token == token
	}

	mac, err := t.ExtraData.signature(token)
	if err != nil {
		return false
	}

	return hmac.Equal(mac, t.Signature)
}

// Encode the token into string.
func (t JoinToken) Encode() (string, error) {
	if t.ExtraData == nil {
		return t.token, nil
	}

	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	return v2Prefix + base64.StdEncoding.EncodeToString(data), nil
}
