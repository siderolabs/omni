// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package constants contains global backend constants.
package constants

import "time"

// SecureBoot defines query parameter for enabling secure boot for the generated image.
// tsgen:SecureBoot
const SecureBoot = "secureboot"

// DefaultTalosVersion is pre-selected in the UI, default image and used in the integration tests.
//
// tsgen:DefaultTalosVersion
const DefaultTalosVersion = "1.6.4"

const (
	// TalosRegistry is the default Talos repository URL.
	TalosRegistry = "ghcr.io/siderolabs/installer"

	// ImageFactoryBaseURL is the default Image Factory base URL.
	ImageFactoryBaseURL = "https://factory.talos.dev"

	// KubernetesRegistry is the default kubernetes repository URL.
	KubernetesRegistry = "ghcr.io/siderolabs/kubelet"
)

const (
	// PatchWeightInstallDisk is the weight of the install disk config patch.
	// tsgen:PatchWeightInstallDisk
	PatchWeightInstallDisk = 0
	// PatchBaseWeightCluster is the base weight for cluster patches.
	// tsgen:PatchBaseWeightCluster
	PatchBaseWeightCluster = 200
	// PatchBaseWeightMachineSet is the base weight for machine set patches.
	// tsgen:PatchBaseWeightMachineSet
	PatchBaseWeightMachineSet = 400
	// PatchBaseWeightClusterMachine is the base weight for cluster machine patches.
	// tsgen:PatchBaseWeightClusterMachine
	PatchBaseWeightClusterMachine = 400
)

const (
	// DefaultAccessGroup specifies the default Kubernetes group asserted in the token claims if the user has modify access to the clusters.
	//
	// If not, the user will only have the groups specified in the ACLs (AccessPolicies) in the token claims (will be empty if there is no matching ACL).
	DefaultAccessGroup = "system:masters"
)

// GRPCMaxMessageSize is the maximum message size for gRPC server.
const GRPCMaxMessageSize = 32 * 1024 * 1024

// DisableValidation force disable resource validation on the Omni runtime for a particular resource (only for debug build).
const DisableValidation = "disable-validation"

const (
	// EncryptionPatchPrefix is the prefix of the encryption config patch.
	EncryptionPatchPrefix = "950"
)

const (
	// EncryptionConfigName human-readable encryption config patch name annotation.
	EncryptionConfigName = "disk encryption config"

	// EncryptionConfigDescription description of the encryption config patch.
	EncryptionConfigDescription = "Makes machine encrypt disks using Omni as a KMS server"
)

// CertificateValidityTime is the default validity time for certificates.
const CertificateValidityTime = time.Hour * 24 * 365 // 1 year

// KubernetesAdminCertCommonName is the common name of the Kubernetes admin certificate.
const KubernetesAdminCertCommonName = "omni:admin"
