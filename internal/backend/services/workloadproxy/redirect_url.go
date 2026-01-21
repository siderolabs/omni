// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// SignedRedirect is the magic added to the signed redirect URL.
// tsgen:SignedRedirect
const SignedRedirect = "v1:"

// DecodeRedirectURL decrypts signed redirect URL.
func DecodeRedirectURL(data string, key []byte) (string, error) {
	if !strings.HasPrefix(data, SignedRedirect) {
		return "", fmt.Errorf("signature not found")
	}

	data = data[len(SignedRedirect):]

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	url, signature, found := strings.Cut(string(raw), "|")
	if !found {
		return "", fmt.Errorf("signature not found")
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(url))

	if !hmac.Equal([]byte(signature), mac.Sum(nil)) {
		return "", fmt.Errorf("signature doesn't match")
	}

	return url, nil
}

// EncodeRedirectURL signs redirect URL.
func EncodeRedirectURL(data string, key []byte) string {
	hmac := hmac.New(sha256.New, key)

	hmac.Write([]byte(data))

	signature := hmac.Sum(nil)

	encoded := append([]byte(data+"|"), signature...)

	return SignedRedirect + base64.StdEncoding.EncodeToString(encoded)
}

// GenKey creates a random signature for the redirect URL signing.
func GenKey() ([]byte, error) {
	secret := make([]byte, 64)

	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
