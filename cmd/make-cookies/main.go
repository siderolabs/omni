// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main provides the entrypoint for the make-cookies binary.
package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"

	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
)

func main() {
	if err := app(); err != nil {
		log.Fatalf("failed to create cookies: %v", err)
	}
}

func app() error {
	_, saKey := serviceaccount.GetFromEnv()
	if saKey == "" {
		return fmt.Errorf("no service account key found in environment variables")
	}

	sa, err := serviceaccount.Decode(saKey)
	if err != nil {
		return fmt.Errorf("error decoding service account key: %w", err)
	}

	keyID := sa.Key.Fingerprint()

	signedIDBytes, err := sa.Key.Sign([]byte(keyID))
	if err != nil {
		return fmt.Errorf("error signing key ID: %w", err)
	}

	cookies := []*http.Cookie{
		{Name: workloadproxy.PublicKeyIDCookie, Value: keyID},
		{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: base64.StdEncoding.EncodeToString(signedIDBytes)},
	}

	for _, cookie := range cookies {
		fmt.Printf("%s\n\n\n", cookie.String())
	}

	return nil
}
