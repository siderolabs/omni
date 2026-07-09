// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"fmt"
	"net/url"
)

// GetPrimaryFactory returns the resolved primary Image Factory configuration.
//
// It applies the "new wins, deprecated fallback" rule: each field uses the value from
// registries.factories.primary when set, otherwise falls back to the deprecated flat
// imageFactory* field (which still carries the default factory URL). This is the factory
// that Omni uses for all its Image Factory operations today.
func (s *Registries) GetPrimaryFactory() Factory {
	primary := s.Factories.Primary

	var resolved Factory

	resolved.SetUrl(firstNonEmpty(primary.GetUrl(), s.GetImageFactoryBaseURL()))
	resolved.SetPxeURL(firstNonEmpty(primary.GetPxeURL(), s.GetImageFactoryPXEBaseURL()))
	resolved.SetUsername(firstNonEmpty(primary.GetUsername(), s.GetImageFactoryUsername()))
	resolved.SetPassword(firstNonEmpty(primary.GetPassword(), s.GetImageFactoryPassword()))

	return resolved
}

// GetSecondaryFactory returns the configured secondary Image Factory configuration and true,
// or a zero Factory and false when no secondary factory is configured.
//
// There is no deprecated fallback for the secondary factory, as it is a new concept.
func (s *Registries) GetSecondaryFactory() (Factory, bool) {
	secondary := s.Factories.Secondary
	if secondary.GetUrl() == "" {
		return Factory{}, false
	}

	return secondary, true
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}
