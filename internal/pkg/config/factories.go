// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"fmt"
	"net/url"
)

// GetPrimaryFactory returns the primary Image Factory configuration. This is the factory that Omni
// uses for all its Image Factory operations today.
func (s *Registries) GetPrimaryFactory() Factory {
	return s.Factories.Primary
}

// GetSecondaryFactory returns the configured secondary Image Factory configuration and true,
// or a zero Factory and false when no secondary factory is configured.
func (s *Registries) GetSecondaryFactory() (Factory, bool) {
	secondary := s.Factories.Secondary
	if secondary.GetUrl() == "" {
		return Factory{}, false
	}

	return secondary, true
}

// AllFactories returns every configured Image Factory: the primary, plus the secondary when one is
// configured. Callers that have to act on each factory in turn should use this rather than pairing
// GetPrimaryFactory with GetSecondaryFactory themselves.
func (s *Registries) AllFactories() []Factory {
	factories := []Factory{s.GetPrimaryFactory()}

	if secondary, ok := s.GetSecondaryFactory(); ok {
		factories = append(factories, secondary)
	}

	return factories
}

// PXEBaseURL returns the PXE base URL for the factory. It uses the explicitly configured
// pxeURL when set, otherwise derives it from the factory URL by prefixing the host with "pxe.".
func (f Factory) PXEBaseURL() (*url.URL, error) {
	if pxe := f.GetPxeURL(); pxe != "" {
		return url.Parse(pxe)
	}

	u, err := url.Parse(f.GetUrl())
	if err != nil {
		return nil, fmt.Errorf("invalid URL specified for the image factory: %w", err)
	}

	u.Host = fmt.Sprintf("pxe.%s", u.Host)

	return u, nil
}

// RequiresAuth returns whether Omni has a token file or basic auth credentials for the factory.
func (f Factory) RequiresAuth() bool {
	return f.GetTokenFile() != "" || f.GetPassword() != ""
}
