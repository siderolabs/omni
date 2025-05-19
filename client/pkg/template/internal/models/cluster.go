// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// KindCluster is Cluster model kind.
const KindCluster = "Cluster"

// Cluster is a top-level template object.
type Cluster struct { //nolint:govet
	Meta             `yaml:",inline"`
	SystemExtensions `yaml:",inline"`

	// Name is the name of the cluster.
	Name string `yaml:"name"`

	// Descriptors are the user descriptors to apply to the cluster.
	Descriptors Descriptors `yaml:",inline"`

	// Kubernetes settings.
	Kubernetes KubernetesCluster `yaml:"kubernetes"`

	// Talos settings.
	Talos TalosCluster `yaml:"talos"`

	// Features settings.
	Features Features `yaml:"features,omitempty"`

	// Cluster-wide patches.
	Patches PatchList `yaml:"patches,omitempty"`
}

// Features defines cluster-wide features.
type Features struct {
	// DiskEncryption enables KMS encryption.
	DiskEncryption bool `yaml:"diskEncryption,omitempty"`
	// EnableWorkloadProxy enables workload proxy.
	EnableWorkloadProxy bool `yaml:"enableWorkloadProxy,omitempty"`
	// UseEmbeddedDiscoveryService enables the use of embedded discovery service.
	UseEmbeddedDiscoveryService bool `yaml:"useEmbeddedDiscoveryService,omitempty"`
	// BackupConfiguration contains backup configuration settings.
	BackupConfiguration BackupConfiguration `yaml:"backupConfiguration,omitempty"`
}

// BackupConfiguration contains backup configuration settings.
type BackupConfiguration struct {
	// Interval configures intervals between backups. If set to 0, etcd backups for this cluster are disabled.
	Interval time.Duration `yaml:"interval,omitempty"`
}

// KubernetesCluster is a Kubernetes cluster settings.
type KubernetesCluster struct {
	// Version is the Kubernetes version.
	Version string `yaml:"version"`
}

// TalosCluster is a Talos cluster settings.
type TalosCluster struct {
	// Version is the Talos version.
	Version string `yaml:"version"`
}

// Validate the model.
func (cluster *Cluster) Validate() error {
	var multiErr error

	validator := omni.ClusterValidator{
		ID:                                cluster.Name,
		KubernetesVersion:                 cluster.Kubernetes.Version,
		TalosVersion:                      cluster.Talos.Version,
		EncryptionEnabled:                 cluster.Features.DiskEncryption,
		RequireVPrefixOnTalosVersionCheck: true,
	}

	if err := validator.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if err := cluster.Descriptors.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if err := cluster.Patches.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if multiErr != nil {
		return fmt.Errorf("error validating cluster %q: %w", cluster.Name, multiErr)
	}

	return nil
}

// Translate into Omni resources.
func (cluster *Cluster) Translate(ctx TranslateContext) ([]resource.Resource, error) {
	clusterResource := omni.NewCluster(resources.DefaultNamespace, cluster.Name)

	clusterResource.Metadata().Annotations().Set(omni.ResourceManagedByClusterTemplates, "")

	cluster.Descriptors.Apply(clusterResource)

	clusterResource.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
		EnableWorkloadProxy:         cluster.Features.EnableWorkloadProxy,
		UseEmbeddedDiscoveryService: cluster.Features.UseEmbeddedDiscoveryService,
	}

	clusterResource.TypedSpec().Value.KubernetesVersion = strings.TrimLeft(cluster.Kubernetes.Version, "v")
	clusterResource.TypedSpec().Value.TalosVersion = strings.TrimLeft(cluster.Talos.Version, "v")

	patches, err := cluster.Patches.Translate(fmt.Sprintf("cluster-%s", cluster.Name), constants.PatchBaseWeightCluster, pair.MakePair(omni.LabelCluster, cluster.Name))
	if err != nil {
		return nil, err
	}

	if clusterResource.TypedSpec().Value.Features == nil {
		clusterResource.TypedSpec().Value.Features = &specs.ClusterSpec_Features{}
	}

	clusterResource.TypedSpec().Value.Features.DiskEncryption = cluster.Features.DiskEncryption

	if interval := cluster.Features.BackupConfiguration.Interval; interval > 0 {
		clusterResource.TypedSpec().Value.BackupConfiguration = &specs.EtcdBackupConf{Interval: durationpb.New(interval), Enabled: true}
	}

	resourceList := append([]resource.Resource{clusterResource}, patches...)

	schematicConfigurations := cluster.translate(
		ctx,
		cluster.Name,
	)

	return append(resourceList, schematicConfigurations...), nil
}

func init() {
	register[Cluster](KindCluster)
}

func joinErrors(err error, errs ...error) error {
	for _, e := range errs {
		if e != nil {
			err = multierror.Append(err, e)
		}
	}

	return err
}
