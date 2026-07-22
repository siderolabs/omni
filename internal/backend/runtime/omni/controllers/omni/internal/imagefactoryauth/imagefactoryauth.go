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

// BuildDoc returns a RegistryAuthConfig doc for the primary image factory based on the
// configured credentials. Returns nil if no credentials are configured.
//
//nolint:nilnil
func BuildDoc(registries omnicfg.Registries) (*cri.RegistryAuthConfigV1Alpha1, error) {
	return buildFactoryDoc(registries.GetPrimaryFactory())
}

// BuildDocs returns RegistryAuthConfig docs for every configured image factory that has
// credentials set: the primary factory and, when configured, the secondary factory.
// Factories without credentials are skipped. Returns nil if none are configured.
func BuildDocs(registries omnicfg.Registries) ([]*cri.RegistryAuthConfigV1Alpha1, error) {
	factories := []omnicfg.Factory{registries.GetPrimaryFactory()}

	if secondary, ok := registries.GetSecondaryFactory(); ok {
		factories = append(factories, secondary)
	}

	var docs []*cri.RegistryAuthConfigV1Alpha1

	for _, factory := range factories {
		doc, err := buildFactoryDoc(factory)
		if err != nil {
			return nil, err
		}

		if doc != nil {
			docs = append(docs, doc)
		}
	}

	return docs, nil
}

// buildFactoryDoc builds a RegistryAuthConfig doc for a single factory, or nil when the
// factory has no credentials configured.
//
//nolint:nilnil
func buildFactoryDoc(factory omnicfg.Factory) (*cri.RegistryAuthConfigV1Alpha1, error) {
	username := factory.GetUsername()
	password := factory.GetPassword()

	if username == "" || password == "" {
		return nil, nil
	}

	u, err := url.Parse(factory.GetUrl())
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory base URL: %w", err)
	}

	doc := cri.NewRegistryAuthConfigV1Alpha1(u.Host)
	doc.RegistryUsername = username
	doc.RegistryPassword = password

	return doc, nil
}
