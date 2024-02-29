// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

const (
	// MachineLocked locks any updates on a machine set node.
	// tsgen:MachineLocked
	MachineLocked = SystemLabelPrefix + "locked"

	// ResourceManagedByClusterTemplates is an annotation which indicates that a resource is managed by cluster templates.
	// tsgen:ResourceManagedByClusterTemplates
	ResourceManagedByClusterTemplates = SystemLabelPrefix + "managed-by-cluster-templates"

	// ConfigPatchName human readable patch name.
	// tsgen:ConfigPatchName
	ConfigPatchName = "name"

	// ConfigPatchDescription human readable patch description.
	// tsgen:ConfigPatchDescription
	ConfigPatchDescription = "description"
)
