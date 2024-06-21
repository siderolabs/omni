// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import "strings"

const (
	serviceAccountDomain              = "serviceaccount.omni.sidero.dev"
	cloudProviderServiceAccountDomain = "cloud-provider." + serviceAccountDomain

	// ServiceAccountNameSuffix is appended to the name of all service accounts.
	ServiceAccountNameSuffix = "@" + serviceAccountDomain

	// CloudProviderServiceAccountPrefix is the prefix required for cloud provider service accounts.
	CloudProviderServiceAccountPrefix = "cloud-provider:"

	// cloudProviderServiceAccountNameSuffix is appended to the name of all cloud provider service accounts.
	cloudProviderServiceAccountNameSuffix = "@" + cloudProviderServiceAccountDomain
)

// ServiceAccount represents a service account.
type ServiceAccount struct {
	// BaseName is the base name of the service account.
	//
	// Example: "aws-1"
	BaseName string

	// Suffix is the suffix of the service account.
	//
	// Example: "@cloud-provider.serviceaccount.omni.sidero.dev"
	Suffix string

	// IsCloudProvider indicates whether the service account is a cloud provider service account.
	IsCloudProvider bool
}

// NameWithPrefix returns the name of the service account with the appropriate prefix.
//
// Example: cloud-provider:aws-1.
func (sa ServiceAccount) NameWithPrefix() string {
	if sa.IsCloudProvider {
		return CloudProviderServiceAccountPrefix + sa.BaseName
	}

	return sa.BaseName
}

// FullID returns the full ID (Identity resource ID / e-mail) of the service account.
//
// Example: aws-1@cloud-provider.serviceaccount.omni.sidero.dev.
func (sa ServiceAccount) FullID() string {
	return sa.BaseName + sa.Suffix
}

// ParseServiceAccountFromName parses a service account from a name with a potential prefix.
//
// Example: name: "cloud-provider:aws-1"
//
// Result: ServiceAccount{BaseName: "aws-1", Suffix: "@cloud-provider.serviceaccount.omni.sidero.dev", IsCloudProvider: true}.
func ParseServiceAccountFromName(name string) ServiceAccount {
	baseName := name
	isCloudProvider := false
	suffix := ServiceAccountNameSuffix

	if strings.HasPrefix(name, CloudProviderServiceAccountPrefix) {
		isCloudProvider = true
		baseName = strings.TrimPrefix(name, CloudProviderServiceAccountPrefix)
		suffix = cloudProviderServiceAccountNameSuffix
	}

	return ServiceAccount{
		BaseName:        baseName,
		Suffix:          suffix,
		IsCloudProvider: isCloudProvider,
	}
}

// ParseServiceAccountFromFullID parses a service account from a full ID (Identity resource ID / e-mail).
//
// Example: fullID: aws-1@cloud-provider.serviceaccount.omni.sidero.dev
//
// Result: ServiceAccount{BaseName: "aws-1", Suffix: "@cloud-provider.serviceaccount.omni.sidero.dev", IsCloudProvider: true}.
func ParseServiceAccountFromFullID(fullID string) (sa ServiceAccount, isSa bool) {
	hasServiceAccountSuffix := strings.HasSuffix(fullID, ServiceAccountNameSuffix)
	hasCloudProviderServiceAccountSuffix := strings.HasSuffix(fullID, cloudProviderServiceAccountNameSuffix)

	if !hasServiceAccountSuffix && !hasCloudProviderServiceAccountSuffix {
		return ServiceAccount{}, false
	}

	isCloudProvider := false
	suffix := ServiceAccountNameSuffix

	if hasCloudProviderServiceAccountSuffix {
		isCloudProvider = true
		suffix = cloudProviderServiceAccountNameSuffix
	}

	return ServiceAccount{
		BaseName:        strings.TrimSuffix(fullID, suffix),
		Suffix:          suffix,
		IsCloudProvider: isCloudProvider,
	}, true
}
