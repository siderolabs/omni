// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

const (
	// MachineLocked locks any updates on a machine set node.
	// tsgen:MachineLocked
	MachineLocked = SystemLabelPrefix + "locked"

	// UpdateLocked machine is locked and has pending update.
	// tsgen:UpdateLocked
	UpdateLocked = SystemLabelPrefix + "locked-update"

	// ResourceManagedByClusterTemplates is an annotation which indicates that a resource is managed by cluster templates.
	// tsgen:ResourceManagedByClusterTemplates
	ResourceManagedByClusterTemplates = SystemLabelPrefix + "managed-by-cluster-templates"

	// ConfigPatchName human readable patch name.
	// tsgen:ConfigPatchName
	ConfigPatchName = "name"

	// ConfigPatchDescription human readable patch description.
	// tsgen:ConfigPatchDescription
	ConfigPatchDescription = "description"

	// PreserveDiskQuotaSupport marks the cluster machine to alter the config generation for it.
	// It forces the config patch that enables diskQuotaSupport feature.
	PreserveDiskQuotaSupport = SystemLabelPrefix + "preserve-disk-quota-support"

	// PreserveApidCheckExtKeyUsage marks the cluster machine to alter the config generation for it.
	// It forces the config patch that enables apidCheckExtkeyUsage feature.
	PreserveApidCheckExtKeyUsage = SystemLabelPrefix + "preserve-apid-check-ext-key-usage"

	// CreatedWithUniqueToken is set on the link resource when it was created with the provision request
	// that has node unique token set.
	CreatedWithUniqueToken = SystemLabelPrefix + "created-with-unique-token"

	// ClusterLocked locks any updates on a cluster.
	// tsgen:ClusterLocked
	ClusterLocked = SystemLabelPrefix + "cluster-locked"

	// ClusterImportIsInProgress indicates that the cluster import process is in progress.
	// This annotation is temporarily set on the cluster during initial import to allow resource creation that's normally blocked by ClusterLocked.
	// tsgen:ClusterImportIsInProgress
	ClusterImportIsInProgress = SystemLabelPrefix + "cluster-import-is-in-progress"
)
