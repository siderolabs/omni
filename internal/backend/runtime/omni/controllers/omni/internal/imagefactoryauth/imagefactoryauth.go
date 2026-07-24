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

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func buildDoc(auth *omni.ImageFactoryAuth) (*cri.RegistryAuthConfigV1Alpha1, error) {
	username := auth.TypedSpec().Value.Username
	password := auth.TypedSpec().Value.Password

	if username == "" || password == "" {
		return nil, nil //nolint:nilnil
	}

	u, err := url.Parse(auth.Metadata().ID())
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory base URL: %w", err)
	}

	doc := cri.NewRegistryAuthConfigV1Alpha1(u.Host)
	doc.RegistryUsername = username
	doc.RegistryPassword = password

	return doc, nil
}

// BuildDocs returns RegistryAuthConfig docs for every configured image factory that has
// credentials set: the primary factory and, when configured, the secondary factory.
// Factories without credentials are skipped. Returns nil if none are configured.
func BuildDocs(creds []*omni.ImageFactoryAuth) ([]*cri.RegistryAuthConfigV1Alpha1, error) {
	if len(creds) == 0 {
		return nil, nil
	}

	var docs []*cri.RegistryAuthConfigV1Alpha1

	for _, auth := range creds {
		doc, err := buildDoc(auth)
		if err != nil {
			return nil, err
		}

		if doc != nil {
			docs = append(docs, doc)
		}
	}

	return docs, nil
}
