// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package imagefactoryauth provides helpers to build a Talos RegistryAuthConfig
// document for authenticating against the configured Omni image factory.
package imagefactoryauth

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"

	omnicfg "github.com/siderolabs/omni/internal/pkg/config"
)

// BuildDoc returns a RegistryAuthConfig doc for the image factory based on the
// configured credentials. Returns nil if no credentials are configured.
//
//nolint:nilnil
func BuildDoc(registries omnicfg.Registries) (*cri.RegistryAuthConfigV1Alpha1, error) {
	username := registries.GetImageFactoryUsername()
	password := registries.GetImageFactoryPassword()

	if username == "" || password == "" {
		return nil, nil
	}

	u, err := url.Parse(registries.GetImageFactoryBaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory base URL: %w", err)
	}

	doc := cri.NewRegistryAuthConfigV1Alpha1(u.Host)
	doc.RegistryUsername = username
	doc.RegistryPassword = password

	return doc, nil
}
