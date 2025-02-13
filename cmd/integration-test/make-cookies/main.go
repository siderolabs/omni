// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main provides the entrypoint for the make-cookies binary.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/siderolabs/omni/cmd/integration-test/pkg/clientconfig"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func main() {
	if err := app(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func app() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// don't forget to build this with the -tags=sidero.debug
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s <endpoint>", os.Args[0])
	}

	cfg := clientconfig.New(os.Args[1])
	defer cfg.Close() //nolint:errcheck

	client, err := cfg.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	keyID, keyIDSignatureBase64, err := clientconfig.RegisterKeyGetIDSignatureBase64(ctx, client)
	if err != nil {
		return fmt.Errorf("error registering key: %w", err)
	}

	cookies := []*http.Cookie{
		{Name: workloadproxy.PublicKeyIDCookie, Value: keyID},
		{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: keyIDSignatureBase64},
	}

	for _, cookie := range cookies {
		fmt.Printf("%s\n\n\n", cookie.String())
	}

	return nil
}
