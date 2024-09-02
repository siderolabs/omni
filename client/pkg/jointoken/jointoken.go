// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jointoken implements siderolink jointoken parser.
package jointoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

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

const v1Prefix = "v1:"

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

	token string

	Signature []byte `json:"signature"`
}

// Parse reads string into token.
func Parse(value string) (JoinToken, error) {
	var res JoinToken

	if strings.HasPrefix(value, v1Prefix) {
		data, err := base64.StdEncoding.DecodeString(strings.TrimLeft(value, v1Prefix))
		if err != nil {
			return res, err
		}

		return res, json.Unmarshal(data, &res)
	}

	res.token = value

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

	return v1Prefix + base64.StdEncoding.EncodeToString(data), nil
}
