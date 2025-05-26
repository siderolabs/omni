// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"strings"
)

const (
	// tsgen:ServiceAccountDomain
	serviceAccountDomain = "serviceaccount.omni.sidero.dev"
	// tsgen:InfraProviderServiceAccountDomain
	infraProviderServiceAccountDomain = "infra-provider." + serviceAccountDomain

	// ServiceAccountNameSuffix is appended to the name of all service accounts.
	ServiceAccountNameSuffix = "@" + serviceAccountDomain

	// InfraProviderServiceAccountPrefix is the prefix required for infra provider service accounts.
	InfraProviderServiceAccountPrefix = "infra-provider:"

	// InfraProviderServiceAccountNameSuffix is appended to the name of all infra provider service accounts.
	InfraProviderServiceAccountNameSuffix = "@" + infraProviderServiceAccountDomain
)

// ServiceAccount represents a service account.
type ServiceAccount struct {
	// BaseName is the base name of the service account.
	//
	// Example: "aws-1"
	BaseName string

	// Suffix is the suffix of the service account.
	//
	// Example: "@infra-provider.serviceaccount.omni.sidero.dev"
	Suffix string

	// IsInfraProvider indicates whether the service account is a infra provider service account.
	IsInfraProvider bool
}

// NameWithPrefix returns the name of the service account with the appropriate prefix.
//
// Example: infra-provider:aws-1.
func (sa ServiceAccount) NameWithPrefix() string {
	if sa.IsInfraProvider {
		return InfraProviderServiceAccountPrefix + sa.BaseName
	}

	return sa.BaseName
}

// FullID returns the full ID (Identity resource ID / e-mail) of the service account.
//
// Example: aws-1@infra-provider.serviceaccount.omni.sidero.dev.
func (sa ServiceAccount) FullID() string {
	return sa.BaseName + sa.Suffix
}

// ParseServiceAccountFromName parses a service account from a name with a potential prefix.
//
// Example: name: "infra-provider:aws-1"
//
// Result: ServiceAccount{BaseName: "aws-1", Suffix: "@infra-provider.serviceaccount.omni.sidero.dev", IsInfraProvider: true}.
func ParseServiceAccountFromName(name string) ServiceAccount {
	baseName := name
	isInfraProvider := false
	suffix := ServiceAccountNameSuffix

	if strings.HasPrefix(name, InfraProviderServiceAccountPrefix) {
		isInfraProvider = true
		baseName = strings.TrimPrefix(name, InfraProviderServiceAccountPrefix)
		suffix = InfraProviderServiceAccountNameSuffix
	}

	return ServiceAccount{
		BaseName:        baseName,
		Suffix:          suffix,
		IsInfraProvider: isInfraProvider,
	}
}

// ParseServiceAccountFromFullID parses a service account from a full ID (Identity resource ID / e-mail).
//
// Example: fullID: aws-1@infra-provider.serviceaccount.omni.sidero.dev
//
// Result: ServiceAccount{BaseName: "aws-1", Suffix: "@infra-provider.serviceaccount.omni.sidero.dev", IsInfraProvider: true}.
func ParseServiceAccountFromFullID(fullID string) (sa ServiceAccount, isSa bool) {
	hasServiceAccountSuffix := strings.HasSuffix(fullID, ServiceAccountNameSuffix)
	hasInfraProviderServiceAccountSuffix := strings.HasSuffix(fullID, InfraProviderServiceAccountNameSuffix)

	if !hasServiceAccountSuffix && !hasInfraProviderServiceAccountSuffix {
		return ServiceAccount{}, false
	}

	isInfraProvider := false
	suffix := ServiceAccountNameSuffix

	if hasInfraProviderServiceAccountSuffix {
		isInfraProvider = true
		suffix = InfraProviderServiceAccountNameSuffix
	}

	return ServiceAccount{
		BaseName:        strings.TrimSuffix(fullID, suffix),
		Suffix:          suffix,
		IsInfraProvider: isInfraProvider,
	}, true
}
