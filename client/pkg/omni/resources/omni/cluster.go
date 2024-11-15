// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewCluster creates new cluster resource.
func NewCluster(ns string, id resource.ID) *Cluster {
	return typed.NewResource[ClusterSpec, ClusterExtension](
		resource.NewMetadata(ns, ClusterType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterSpec{}),
	)
}

const (
	// ClusterType is the type of the Cluster resource.
	// tsgen:ClusterType
	ClusterType = resource.Type("Clusters.omni.sidero.dev")
)

// Cluster describes cluster resource.
type Cluster = typed.Resource[ClusterSpec, ClusterExtension]

// ClusterSpec wraps specs.ClusterSpec.
type ClusterSpec = protobuf.ResourceSpec[specs.ClusterSpec, *specs.ClusterSpec]

// ClusterExtension provides auxiliary methods for Cluster resource.
type ClusterExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// GetInstallImage extracts Talos version from the cluster resource and adds the installer image url.
func GetInstallImage(vanillaInstallerBaseURL, factoryBaseURL, schematic, version string) (string, error) {
	version = strings.TrimLeft(version, "v")

	if schematic == "" {
		return fmt.Sprintf("%s:v%s", vanillaInstallerBaseURL, version), nil
	}

	u, err := url.Parse(factoryBaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse image factory url %w", err)
	}

	return filepath.Join(u.Host, "installer", fmt.Sprintf("%s:v%s", schematic, version)), nil
}

// GetEncryptionEnabled returns cluster disk encryption feature flag state.
func GetEncryptionEnabled(cluster *Cluster) bool {
	return cluster.TypedSpec().Value.Features != nil && cluster.TypedSpec().Value.Features.DiskEncryption
}

// GetManagedEnabled returns managed mode flag state.
func GetManagedEnabled(cluster *Cluster) bool {
	return cluster.TypedSpec().Value.Features != nil && cluster.TypedSpec().Value.Features.UseManagedControlPlanes
}
