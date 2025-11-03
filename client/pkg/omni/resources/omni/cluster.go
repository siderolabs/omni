// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/hashicorp/go-multierror"

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
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Talos Version",
				JSONPath: "{.talosversion}",
			},
			{
				Name:     "Kubernetes Version",
				JSONPath: "{.kubernetesversion}",
			},
		},
	}
}

// GetEncryptionEnabled returns cluster disk encryption feature flag state.
func GetEncryptionEnabled(cluster *Cluster) bool {
	return cluster.TypedSpec().Value.Features != nil && cluster.TypedSpec().Value.Features.DiskEncryption
}

// ClusterValidator runs validations which do not require the information from the server (e.g., client-side validations) for the provided cluster properties.
type ClusterValidator struct {
	ID                string
	KubernetesVersion string
	TalosVersion      string
	EncryptionEnabled bool

	// SkipClusterIDCheck indicates if the cluster ID check should be skipped. For example, this is used on server-side update validations.
	SkipClusterIDCheck bool

	// SkipTalosVersionCheck indicates if the Talos version check should be skipped. For example, this is used on server-side update validations.
	SkipTalosVersionCheck bool

	// SkipKubernetesVersionCheck indicates if the Kubernetes version check should be skipped. For example, this is used on server-side update validations.
	SkipKubernetesVersionCheck bool

	// RequireVPrefixOnTalosVersionCheck indicates if the Talos version should start with 'v'. For example, this is the format required by the cluster templates.
	RequireVPrefixOnTalosVersionCheck bool
}

// Validate runs validations on the cluster properties.
func (validator ClusterValidator) Validate() error {
	var multiErr error

	if err := validator.validateID(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if !validator.SkipKubernetesVersionCheck {
		if err := validator.validateKubernetesVersion(); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	if !validator.SkipTalosVersionCheck {
		if err := validator.validateTalosVersion(); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	if err := validator.validateEncryption(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	return multiErr
}

func (validator ClusterValidator) validateID() error {
	if validator.SkipClusterIDCheck {
		return nil
	}

	var multiErr error

	id := validator.ID

	if id == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("name is required"))
	}

	for _, c := range id {
		if !unicode.IsDigit(c) && !unicode.IsLetter(c) && c != '-' && c != '_' {
			multiErr = multierror.Append(multiErr, fmt.Errorf("name should only contain letters, digits, dashes and underscores"))

			break
		}
	}

	return multiErr
}

func (validator ClusterValidator) validateKubernetesVersion() error {
	var multiErr error

	if validator.KubernetesVersion == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version is required"))
	} else if _, err := semver.ParseTolerant(validator.KubernetesVersion); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version should be in semver format: %w", err))
	}

	if multiErr != nil {
		return fmt.Errorf("error validating Kubernetes version: %w", multiErr)
	}

	return nil
}

func (validator ClusterValidator) validateTalosVersion() error {
	var multiErr error

	if validator.TalosVersion == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version is required"))
	} else {
		if _, err := semver.ParseTolerant(validator.TalosVersion); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("version should be in semver format: %w", err))
		}

		if validator.RequireVPrefixOnTalosVersionCheck {
			if !strings.HasPrefix(validator.TalosVersion, "v") {
				multiErr = multierror.Append(multiErr, fmt.Errorf("version should start with 'v'"))
			}
		}
	}

	if multiErr != nil {
		return fmt.Errorf("error validating Talos version: %w", multiErr)
	}

	return nil
}

func (validator ClusterValidator) validateEncryption() error {
	if !validator.EncryptionEnabled {
		return nil
	}

	var (
		encryptionSupport = semver.MustParse("1.5.0")
		version           semver.Version
		err               error
	)
	if version, err = semver.ParseTolerant(validator.TalosVersion); err != nil {
		return err
	}

	if version.Compare(encryptionSupport) < 0 {
		return errors.New("disk encryption is supported only for Talos version >= 1.5.0")
	}

	return nil
}
