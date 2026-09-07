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

// MachineTokenUsername is the basic auth username the machines send along with the machine token.
//
// The factory takes an API token as the basic auth password and ignores the username, so its only job
// is to say what the password is when someone reads the machine config.
const MachineTokenUsername = "omni-machine-token"

func buildDoc(auth *omni.ImageFactoryAuth) (*cri.RegistryAuthConfigV1Alpha1, error) {
	spec := auth.TypedSpec().Value

	// When using an authenticated factory, Omni has no credentials for the machines until the machine
	// token exists: a config generated without them would only fail the installer pull.
	if spec.ApiToken != "" && spec.MachineToken == "" {
		return nil, fmt.Errorf("the machine token for the image factory %q is not available yet", auth.Metadata().ID())
	}

	username, password := spec.Username, spec.Password

	// The machine token wins over basic auth credentials. Both are never set at the same time, since a
	// factory takes one or the other, but the token is the one meant for the machines.
	if spec.MachineToken != "" {
		username, password = MachineTokenUsername, spec.MachineToken
	}

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
// credentials for the machines: the machine token when Omni authenticates with an API token,
// the basic auth credentials otherwise. Factories without credentials are skipped, a factory
// whose machine token does not exist yet is an error. Returns nil if none are configured.
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
