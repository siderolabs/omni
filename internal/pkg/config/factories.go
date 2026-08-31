// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"fmt"
	"net/url"
)

// GetPrimaryFactory returns the resolved primary Image Factory configuration. This is the factory
// that Omni uses for all its Image Factory operations today.
//
// A registries.factories.primary block that specifies a URL is self-contained: the deprecated flat
// imageFactory* fields are ignored entirely. Otherwise credentials configured for the old factory
// would be sent to a newly configured one, which is both wrong and a way to leak them.
//
// Without a URL under registries.factories.primary, the deprecated fields are used instead (they
// still carry the default factory URL), with any field set on the primary block winning.
func (s *Registries) GetPrimaryFactory() Factory {
	primary := s.Factories.Primary

	if primary.GetUrl() != "" {
		return primary
	}

	var resolved Factory

	resolved.SetUrl(s.GetImageFactoryBaseURL())
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

// RequiresAuth returns whether the factory requires auth.
func (f Factory) RequiresAuth() bool {
	return f.GetPassword() != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}
