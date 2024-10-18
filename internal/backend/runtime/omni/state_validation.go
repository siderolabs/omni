// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// clusterValidationOptions returns the validation options for the Talos and Kubernetes versions on the cluster resource.
// Validation is only syntactic - they are checked whether they are valid semver strings.
//
//nolint:gocognit,gocyclo,cyclop
func clusterValidationOptions(st state.State, etcdBackupConfig config.EtcdBackupParams, embeddedDiscoveryServiceConfig config.EmbeddedDiscoveryServiceParams) []validated.StateOption {
	validateVersions := func(ctx context.Context, existingRes *omni.Cluster, res *omni.Cluster, skipTalosVersion, skipKubernetesVersion bool) error {
		if skipTalosVersion && skipKubernetesVersion {
			return nil
		}

		talosVersion, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(resources.DefaultNamespace, res.TypedSpec().Value.TalosVersion).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) && skipTalosVersion {
				return nil
			}

			return fmt.Errorf("invalid talos version %q: %w", res.TypedSpec().Value.TalosVersion, err)
		}

		var currentTalosVersion string

		if existingRes != nil {
			currentTalosVersion = existingRes.TypedSpec().Value.TalosVersion
		}

		if err = validateTalosVersion(ctx, st, currentTalosVersion, res.TypedSpec().Value.TalosVersion); err != nil {
			return err
		}

		if skipKubernetesVersion {
			return nil
		}

		for _, compatibleKubernetesVersion := range talosVersion.TypedSpec().Value.CompatibleKubernetesVersions {
			if compatibleKubernetesVersion == res.TypedSpec().Value.KubernetesVersion {
				return nil
			}
		}

		return fmt.Errorf("invalid kubernetes version %q: is not compatible with talos version %q", res.TypedSpec().Value.KubernetesVersion, res.TypedSpec().Value.TalosVersion)
	}

	encryptionSupport := semver.MustParse("1.5.0")

	validateEncryption := func(res *omni.Cluster) error {
		if !omni.GetEncryptionEnabled(res) {
			return nil
		}

		var (
			version semver.Version
			err     error
		)

		if version, err = semver.ParseTolerant(res.TypedSpec().Value.TalosVersion); err != nil {
			return err
		}

		if version.Compare(encryptionSupport) < 0 {
			return errors.New("disk encryption is supported only for Talos version >= 1.5.0")
		}

		return nil
	}

	validateBackupInterval := func(res *omni.Cluster) error {
		if conf := res.TypedSpec().Value.GetBackupConfiguration(); conf != nil {
			switch conf := conf.GetInterval().AsDuration(); {
			case conf < etcdBackupConfig.MinInterval:
				return fmt.Errorf(
					"backup interval must be greater than %s, actual %s",
					etcdBackupConfig.MinInterval.String(),
					conf.String(),
				)
			case conf > etcdBackupConfig.MaxInterval:
				return fmt.Errorf(
					"backup interval must be less than %s, actual %s",
					etcdBackupConfig.MaxInterval.String(),
					conf.String(),
				)
			}
		}

		return nil
	}

	validateEmbeddedDiscoveryServiceSetting := func(oldRes, newRes *omni.Cluster) error {
		newValue := newRes.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService()
		if !newValue { // feature being disabled is always valid
			return nil
		}

		// if this is a create operation or if the setting is changed, validate that the feature is available
		if oldRes == nil || oldRes.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService() != newValue {
			if !embeddedDiscoveryServiceConfig.Enabled {
				return errors.New("embedded discovery service is not enabled")
			}
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.Cluster, _ ...state.CreateOption) error {
			var multiErr error

			if err := validateEncryption(res); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateBackupInterval(res); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateEmbeddedDiscoveryServiceSetting(nil, res); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateVersions(ctx, nil, res, false, false); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			return multiErr
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, existingRes *omni.Cluster, newRes *omni.Cluster, _ ...state.UpdateOption) error {
			if existingRes == nil {
				// shouldn't happen - skip the validation, so that the original error (NotFound) will be returned
				return nil
			}

			var multiErr error

			skipTalosVersion := existingRes.TypedSpec().Value.TalosVersion == newRes.TypedSpec().Value.TalosVersion
			skipKubernetesVersion := existingRes.TypedSpec().Value.KubernetesVersion == newRes.TypedSpec().Value.KubernetesVersion

			if omni.GetEncryptionEnabled(existingRes) != omni.GetEncryptionEnabled(newRes) {
				multiErr = multierror.Append(multiErr, errors.New("updating disk encryption settings is not allowed"))
			}

			if err := validateBackupInterval(newRes); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateEmbeddedDiscoveryServiceSetting(existingRes, newRes); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateVersions(ctx, existingRes, newRes, skipTalosVersion, skipKubernetesVersion); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			return multiErr
		})),
	}
}

// relationLabelsValidationOptions returns the validation options for the relation labels on the resources.
func relationLabelsValidationOptions() []validated.StateOption {
	validateLabelIsSet := func(res resource.Resource, key string) error {
		val, ok := res.Metadata().Labels().Get(key)
		if !ok {
			return fmt.Errorf("label %q does not exist", key)
		}

		if val == "" {
			return fmt.Errorf("label %q has empty value", key)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineSetNode, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelMachineSet)
			}),
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.MachineSet, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelCluster)
			}),
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.ExposedService, _ ...state.CreateOption) error {
				return validateLabelIsSet(res, omni.LabelCluster)
			}),
		),
		validated.WithUpdateValidations(
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelMachineSet)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.MachineSet, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelCluster)
			}),
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.ExposedService, newRes *omni.ExposedService, _ ...state.UpdateOption) error {
				return validateLabelIsSet(newRes, omni.LabelCluster)
			}),
		),
	}
}

// accessPolicyValidationOptions returns the validation options for the access policy resource.
func accessPolicyValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.AccessPolicy, _ ...state.CreateOption) error {
			return accesspolicy.Validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.AccessPolicy, newRes *authres.AccessPolicy, _ ...state.UpdateOption) error {
			return accesspolicy.Validate(newRes)
		})),
	}
}

// roleValidationOptions returns the validation options for the user and public key resources, ensuring that their roles are valid.
func roleValidationOptions() []validated.StateOption {
	validateRole := func(roleStr string) error {
		_, err := role.Parse(roleStr)

		return err
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.User, _ ...state.CreateOption) error {
			return validateRole(res.TypedSpec().Value.GetRole())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.User, newRes *authres.User, _ ...state.UpdateOption) error {
			return validateRole(newRes.TypedSpec().Value.GetRole())
		})),
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.PublicKey, _ ...state.CreateOption) error {
			return validateRole(res.TypedSpec().Value.GetRole())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.PublicKey, newRes *authres.PublicKey, _ ...state.UpdateOption) error {
			return validateRole(newRes.TypedSpec().Value.GetRole())
		})),
	}
}

// machineSetValidationOptions returns the validation options for the machine set resource.
//
//nolint:gocognit,gocyclo,cyclop
func machineSetValidationOptions(st state.State, etcdBackupStoreFactory store.Factory) []validated.StateOption {
	validate := func(ctx context.Context, oldRes *omni.MachineSet, res *omni.MachineSet) error {
		// label validations
		clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return errors.New("cluster label is missing")
		}

		if oldRes == nil {
			cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}

			if cluster != nil && cluster.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("the cluster %q is tearing down", clusterName)
			}
		}

		_, isControlPlane := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		_, isWorker := res.Metadata().Labels().Get(omni.LabelWorkerRole)

		if !isControlPlane && !isWorker {
			return fmt.Errorf("machine set must have either %q or %q label", omni.LabelControlPlaneRole, omni.LabelWorkerRole)
		}

		if isControlPlane && oldRes == nil { // creating a new control plane machine set
			bootstrapStatus, err := safe.StateGetByID[*omni.ClusterBootstrapStatus](ctx, st, clusterName)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting cluster bootstrap status: %w", err)
			}

			if bootstrapStatus != nil && bootstrapStatus.TypedSpec().Value.GetBootstrapped() {
				return errors.New("adding control plane machine set to an already bootstrapped cluster is not allowed")
			}
		}

		if err := validateBootstrapSpec(ctx, st, etcdBackupStoreFactory, oldRes, res); err != nil {
			return err
		}

		allocationConfig := omni.GetMachineAllocation(res)

		if allocationConfig != nil {
			if allocationConfig.Name == "" {
				return errors.New("machine allocation source name is not set")
			}

			if allocationConfig.MachineCount != 0 && allocationConfig.AllocationType != specs.MachineSetSpec_MachineAllocation_Static {
				return errors.New("machine count can be set only if static allocation type is used")
			}

			var oldAllocationConfig *specs.MachineSetSpec_MachineAllocation

			if oldRes != nil {
				oldAllocationConfig = omni.GetMachineAllocation(oldRes)
			}

			// if change machine class, verify the specified class name exists.
			changed := oldRes == nil || oldAllocationConfig != nil && oldAllocationConfig.Name != allocationConfig.Name
			if changed {
				switch allocationConfig.Source {
				case specs.MachineSetSpec_MachineAllocation_MachineRequestSet:
					_, err := st.Get(ctx, omni.NewMachineRequestSet(resources.DefaultNamespace, allocationConfig.Name).Metadata())
					if err != nil {
						if state.IsNotFoundError(err) {
							return fmt.Errorf("machine request set with name %q doesn't exist", allocationConfig.Name)
						}

						return err
					}
				case specs.MachineSetSpec_MachineAllocation_MachineClass:
					mc, err := safe.ReaderGetByID[*omni.MachineClass](ctx, st, allocationConfig.Name)
					if err != nil {
						if state.IsNotFoundError(err) {
							return fmt.Errorf("machine class with name %q doesn't exist", allocationConfig.Name)
						}

						return err
					}

					if mc.TypedSpec().Value.AutoProvision != nil && allocationConfig.AllocationType == specs.MachineSetSpec_MachineAllocation_Unlimited {
						return fmt.Errorf("machine class %q is using autoprovision, so unlimited machine set allocation is not supported", allocationConfig.Name)
					}
				}
			}
		}

		if oldRes != nil {
			// ensure that the machine class type doesn't change from manually selected machines to the machine class
			oldAllocationConfig := omni.GetMachineAllocation(oldRes)
			newAllocationConfig := omni.GetMachineAllocation(res)

			mgmtModeSwitchedToMachineClass := oldAllocationConfig == nil && newAllocationConfig != nil
			mgmtModeSwitchedToManual := oldAllocationConfig != nil && newAllocationConfig == nil
			mgmtModeSwitchedSource := oldAllocationConfig != nil && newAllocationConfig != nil && oldAllocationConfig.Source != newAllocationConfig.Source
			mgmtModeChanged := mgmtModeSwitchedToMachineClass || mgmtModeSwitchedToManual || mgmtModeSwitchedSource

			if mgmtModeChanged {
				machineSetNodeList, err := safe.StateListAll[*omni.MachineSetNode](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, res.Metadata().ID())))
				if err != nil {
					return fmt.Errorf("error getting machine set nodes: %w", err)
				}

				// block management mode change only if there are nodes in the machine set
				if machineSetNodeList.Len() > 0 {
					switch {
					case mgmtModeSwitchedSource:
						return errors.New("machine set is not empty, updating source is not allowed")
					case mgmtModeSwitchedToMachineClass:
						return errors.New("machine set is not empty and is using manual nodes management, updating to machine class mode is not allowed")
					case mgmtModeSwitchedToManual:
						return errors.New("machine set is not empty and is using machine class based node management, updating to manual mode is not allowed")
					}
				}
			}

			return nil
		}

		// id validations
		clusterPrefix := clusterName + "-"

		if !strings.HasPrefix(res.Metadata().ID(), clusterPrefix) {
			return fmt.Errorf("machine set of cluster %q ID must have %q as prefix", clusterName, clusterPrefix)
		}

		cpID := omni.ControlPlanesResourceID(clusterName)

		if isControlPlane {
			if res.Metadata().ID() == cpID {
				return nil
			}

			return fmt.Errorf("control plane machine set must have ID %q", cpID)
		}

		if res.Metadata().ID() == cpID {
			return fmt.Errorf("worker machine set must not have ID %q", cpID)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineSet, _ ...state.CreateOption) error {
			return validate(ctx, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineSet, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
			return validate(ctx, oldRes, newRes)
		})),
	}
}

// machineClassValidationOptions returns the validation options for the machine class resource.
func machineClassValidationOptions(st state.State) []validated.StateOption {
	validate := func(ctx context.Context, oldRes, res *omni.MachineClass) error {
		if res.TypedSpec().Value.AutoProvision != nil && res.TypedSpec().Value.MatchLabels != nil {
			return errors.New("can't set both auto provision and match labels at the same time")
		}

		if res.TypedSpec().Value.AutoProvision != nil {
			autoProvision := res.TypedSpec().Value.AutoProvision

			if autoProvision.ProviderId == "" {
				return errors.New("providerID can not be empty")
			}

			if oldRes == nil || oldRes.TypedSpec().Value.AutoProvision.ProviderData != autoProvision.ProviderData {
				if err := validateProviderData(ctx, st, autoProvision.ProviderId, autoProvision.ProviderData); err != nil {
					return err
				}
			}

			return nil
		}

		queries, err := labels.ParseSelectors(res.TypedSpec().Value.MatchLabels)
		if err != nil {
			return fmt.Errorf("failed to parse matchLabels: %w", err)
		}

		if len(queries) == 0 {
			return fmt.Errorf("machine class should either have auto provision or match labels set")
		}

		if slices.IndexFunc(queries, func(s resource.LabelQuery) bool {
			return slices.IndexFunc(s.Terms, func(term resource.LabelTerm) bool {
				return term.Key == omni.LabelNoManualAllocation
			}) != -1
		}) != -1 {
			return fmt.Errorf("selectors using label %s are not allowed", omni.LabelNoManualAllocation)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineClass, _ ...state.CreateOption) error {
			return validate(ctx, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineClass, res *omni.MachineClass, _ ...state.UpdateOption) error {
			return validate(ctx, oldRes, res)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.MachineClass, _ ...state.DestroyOption) error {
			machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, st)
			if err != nil {
				return err
			}

			var inUseBy []string

			machineSets.ForEach(func(r *omni.MachineSet) {
				if alloc := omni.GetMachineAllocation(r); alloc != nil && alloc.Source == specs.MachineSetSpec_MachineAllocation_MachineClass && res.Metadata().ID() == alloc.Name {
					inUseBy = append(inUseBy, r.Metadata().ID())
				}
			})

			if len(inUseBy) > 0 {
				return fmt.Errorf("can not delete the machine class as it is still in use by machine sets: %s", strings.Join(inUseBy, ", "))
			}

			return nil
		})),
	}
}

func validateBootstrapSpec(ctx context.Context, st state.State, etcdBackupStoreFactory store.Factory, oldres, res *omni.MachineSet) error {
	bootstrapSpec := res.TypedSpec().Value.GetBootstrapSpec()
	_, isControlPlane := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	if !isControlPlane && bootstrapSpec != nil {
		return errors.New("bootstrap spec is not allowed for worker machine sets")
	}

	if oldres != nil { // this is an update
		if !bootstrapSpec.EqualVT(oldres.TypedSpec().Value.GetBootstrapSpec()) {
			return errors.New("bootstrap spec is immutable after creation")
		}

		// short-circuit if the bootstrap spec is not changed on update - it was already validated on creation
		return nil
	}

	if bootstrapSpec == nil {
		return nil
	}

	clusterUUIDs, err := safe.StateListAll[*omni.ClusterUUID](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterUUID, bootstrapSpec.GetClusterUuid())))
	if err != nil {
		return fmt.Errorf("error getting cluster UUIDs: %w", err)
	}

	if clusterUUIDs.Len() == 0 {
		return fmt.Errorf("invalid cluster UUID %q", bootstrapSpec.GetClusterUuid())
	}

	if clusterUUIDs.Len() > 1 {
		return fmt.Errorf("inconsistent state on cluster UUID %q", bootstrapSpec.GetClusterUuid())
	}

	cluster := clusterUUIDs.Get(0).Metadata().ID()

	backupData, err := safe.ReaderGetByID[*omni.BackupData](ctx, st, cluster)
	if err != nil {
		return fmt.Errorf("error getting backup data: %w", err)
	}

	backupStore, err := etcdBackupStoreFactory.GetStore()
	if err != nil {
		return fmt.Errorf("error getting etcd backup store: %w", err)
	}

	data, readCloser, err := backupStore.Download(ctx, backupData.TypedSpec().Value.EncryptionKey, bootstrapSpec.ClusterUuid, bootstrapSpec.Snapshot)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}

	readCloser.Close() //nolint:errcheck

	if data.AESCBCEncryptionSecret != backupData.TypedSpec().Value.AesCbcEncryptionSecret {
		return errors.New("aes cbc encryption secret mismatch")
	}

	if data.SecretboxEncryptionSecret != backupData.TypedSpec().Value.SecretboxEncryptionSecret {
		return errors.New("secretbox encryption secret mismatch")
	}

	return nil
}

// machineSetNodeValidationOptions returns the validation options for the machine set node resource.
//
//nolint:gocognit,gocyclo,cyclop
func machineSetNodeValidationOptions(st state.State) []validated.StateOption {
	getMachineSet := func(ctx context.Context, res *omni.MachineSetNode) (*omni.MachineSet, error) {
		machineSetName, ok := res.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return nil, nil //nolint:nilnil
		}

		machineSet, err := safe.ReaderGet[*omni.MachineSet](ctx, st, omni.NewMachineSet(resources.DefaultNamespace, machineSetName).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil //nolint:nilnil
			}

			return nil, err
		}

		return machineSet, nil
	}

	validateTalosVersion := func(ctx context.Context, res *omni.MachineSetNode) error {
		clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return nil
		}

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, st, res.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		machineTalosVersion, err := semver.Parse(strings.TrimLeft(machineStatus.TypedSpec().Value.TalosVersion, "v"))
		if err != nil {
			// ignore version check if it's not possible to parse machine Talos version
			return nil //nolint:nilerr
		}

		clusterTalosVersion, err := semver.Parse(cluster.TypedSpec().Value.TalosVersion)
		if err != nil {
			return err
		}

		if machineTalosVersion.Major > clusterTalosVersion.Major || machineTalosVersion.Minor > clusterTalosVersion.Minor {
			return fmt.Errorf(
				"cannot add machine set node to the cluster %s as it will trigger Talos downgrade on the node (%s -> %s)",
				clusterName,
				machineTalosVersion.String(),
				clusterTalosVersion.String(),
			)
		}

		installed := omni.GetMachineStatusSystemDisk(machineStatus) != ""

		if !installed && (machineTalosVersion.Major != clusterTalosVersion.Major || machineTalosVersion.Minor != clusterTalosVersion.Minor) {
			return errors.New(
				"machines which are running Talos without installation can be added only to Talos clusters with the same major and minor versions",
			)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineSetNode, _ ...state.CreateOption) error {
			machineSet, err := getMachineSet(ctx, res)
			if err != nil {
				return err
			}

			if machineSet != nil && machineSet.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("the machine set %q is tearing down", machineSet.Metadata().ID())
			}

			if machineSet != nil && omni.GetMachineAllocation(machineSet) != nil {
				return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the machine set is using automated machine allocation", machineSet.Metadata().ID())
			}

			if err = validateTalosVersion(ctx, res); err != nil {
				return err
			}

			return validateNotControlplane(machineSet, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, res *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
			// don't allow tearing down machine set nodes with locked annotation
			if newRes.Metadata().Phase() == resource.PhaseTearingDown {
				if _, locked := res.Metadata().Annotations().Get(omni.MachineLocked); locked {
					return errors.New("machine set node is locked")
				}
			}

			machineSet, err := getMachineSet(ctx, res)
			if err != nil {
				return err
			}

			return validateNotControlplane(machineSet, newRes)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.MachineSetNode, _ ...state.DestroyOption) error {
			machineSetName, ok := res.Metadata().Labels().Get(omni.LabelMachineSet)
			if ok {
				machineSet, err := safe.StateGet[*omni.MachineSet](ctx, st, omni.NewMachineSet(resources.DefaultNamespace, machineSetName).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				// if the machine set is being torn down or doesn't exist disable machine locks
				if machineSet == nil || machineSet.Metadata().Phase() == resource.PhaseTearingDown {
					return nil
				}
			}

			if _, locked := res.Metadata().Annotations().Get(omni.MachineLocked); locked {
				return errors.New("machine set node is locked")
			}

			return nil
		})),
	}
}

func schematicConfigurationValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(
			func(_ context.Context, res *omni.SchematicConfiguration, _ ...state.CreateOption) error {
				return validateSchematicConfiguration(res)
			},
		)),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(_ context.Context, _, res *omni.SchematicConfiguration, _ ...state.UpdateOption) error {
				return validateSchematicConfiguration(res)
			},
		)),
	}
}

func hasUppercaseLetters(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return true
		}
	}

	return false
}

func identityValidationOptions(samlConfig config.SAMLParams) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *authres.Identity, _ ...state.CreateOption) error {
			var errs error

			if hasUppercaseLetters(res.Metadata().ID()) {
				errs = multierror.Append(errs, errors.New("must be lowercase"))
			}

			// allow non-email identities for internal actors and for users coming from the SAML provider
			if samlConfig.Enabled || actor.ContextIsInternalActor(ctx) {
				return nil
			}

			if _, err := mail.ParseAddress(res.Metadata().ID()); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("not a valid email address: %s", res.Metadata().ID()))
			}

			return errs
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, res *authres.Identity, newRes *authres.Identity, _ ...state.UpdateOption) error {
			if !samlConfig.Enabled || actor.ContextIsInternalActor(ctx) {
				return nil
			}

			changed := newRes.TypedSpec().Value.UserId != res.TypedSpec().Value.UserId ||
				!newRes.Metadata().Labels().Equal(*res.Metadata().Labels()) ||
				!newRes.Metadata().Annotations().Equal(*res.Metadata().Annotations())

			if changed {
				return errors.New("updating identity is not allowed in SAML mode")
			}

			return nil
		})),
	}
}

func exposedServiceValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.ExposedService, _ ...state.CreateOption) error {
			alias, _ := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
			if alias == "" {
				return errors.New("alias must be set")
			}

			return nil
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, res *omni.ExposedService, newRes *omni.ExposedService, _ ...state.UpdateOption) error {
			oldAlias, _ := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
			newAlias, _ := newRes.Metadata().Labels().Get(omni.LabelExposedServiceAlias)

			if oldAlias != newAlias {
				return errors.New("alias cannot be changed")
			}

			return nil
		})),
	}
}

func configPatchValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.ConfigPatch, _ ...state.CreateOption) error {
			if clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster); ok {
				cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if cluster != nil && cluster.Metadata().Phase() == resource.PhaseTearingDown {
					return fmt.Errorf("cluster %q is tearing down", clusterName)
				}
			}

			if machineSetName, ok := res.Metadata().Labels().Get(omni.LabelMachineSet); ok {
				machineSet, err := safe.StateGetByID[*omni.MachineSet](ctx, st, machineSetName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if machineSet != nil && machineSet.Metadata().Phase() == resource.PhaseTearingDown {
					return fmt.Errorf("machine set %q is tearing down", machineSetName)
				}
			}

			buffer, err := res.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer buffer.Free()

			return omni.ValidateConfigPatch(buffer.Data())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.ConfigPatch, newRes *omni.ConfigPatch, _ ...state.UpdateOption) error {
			// keep the old config patch if the data is the same for backwards-compatibility and for teardown cases
			oldBuffer, err := oldRes.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer oldBuffer.Free()

			newBuffer, err := newRes.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer newBuffer.Free()

			oldData := oldBuffer.Data()
			newData := newBuffer.Data()

			if bytes.Equal(oldData, newData) {
				return nil
			}

			return omni.ValidateConfigPatch(newData)
		})),
	}
}

func validateNotControlplane(machineSet *omni.MachineSet, res *omni.MachineSetNode) error {
	if _, locked := res.Metadata().Annotations().Get(omni.MachineLocked); !locked {
		return nil
	}

	if machineSet == nil {
		return nil
	}

	if _, cp := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); cp {
		return errors.New("locking controlplanes is not allowed")
	}

	return nil
}

func etcdManualBackupValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.EtcdManualBackup, _ ...state.CreateOption) error {
			return validateManualBackup(res.TypedSpec())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.EtcdManualBackup, newRes *omni.EtcdManualBackup, _ ...state.UpdateOption) error {
			if oldRes == nil {
				return nil
			}

			if oldRes.TypedSpec().Value.BackupAt.AsTime().Equal(newRes.TypedSpec().Value.BackupAt.AsTime()) {
				return nil
			}

			return validateManualBackup(newRes.TypedSpec())
		})),
	}
}

// TODO: maybe move the role validation into roleValidationOptions and create a "matchLabelsValidationOptions" function.
func samlLabelRuleValidationOptions() []validated.StateOption {
	validate := func(res *authres.SAMLLabelRule) error {
		var multiErr error

		if _, err := role.Parse(res.TypedSpec().Value.GetAssignRoleOnRegistration()); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}

		if _, err := labels.ParseSelectors(res.TypedSpec().Value.GetMatchLabels()); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid match labels: %w", err))
		}

		return multiErr
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *authres.SAMLLabelRule, _ ...state.CreateOption) error {
			return validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _ *authres.SAMLLabelRule, newRes *authres.SAMLLabelRule, _ ...state.UpdateOption) error {
			return validate(newRes)
		})),
	}
}

func validateManualBackup(embs *omni.EtcdManualBackupSpec) error {
	backupAt := embs.Value.GetBackupAt().AsTime()

	if time.Since(backupAt) > time.Minute {
		return errors.New("backup time must not be more than 1 minute in the past")
	} else if time.Until(backupAt) > time.Minute {
		return errors.New("backup time must not be more than 1 minute in the future")
	}

	return nil
}

func s3ConfigValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.EtcdBackupS3Conf, _ ...state.CreateOption) error {
			return validateS3Configuration(ctx, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, _ *omni.EtcdBackupS3Conf, newRes *omni.EtcdBackupS3Conf, _ ...state.UpdateOption) error {
			return validateS3Configuration(ctx, newRes)
		})),
	}
}

func validateS3Configuration(ctx context.Context, s3Conf *omni.EtcdBackupS3Conf) error {
	if store.IsEmptyS3Conf(s3Conf) {
		return nil
	}

	_, _, err := store.S3ClientFromResource(ctx, s3Conf)
	if err != nil {
		return fmt.Errorf("incorrect settings for s3 client: %w", err)
	}

	return nil
}

func validateSchematicConfiguration(schematicConfiguration *omni.SchematicConfiguration) error {
	var targetValid bool

	labels := []string{
		omni.LabelClusterMachine,
		omni.LabelMachineSet,
		omni.LabelCluster,
	}

	for _, label := range labels {
		_, targetValid = schematicConfiguration.Metadata().Labels().Get(label)
		if targetValid {
			break
		}
	}

	if !targetValid {
		return fmt.Errorf("schematic configuration should have one of %q labels", strings.Join(labels, ", "))
	}

	if schematicConfiguration.TypedSpec().Value.SchematicId == "" {
		return fmt.Errorf("schematic ID can not be empty")
	}

	return nil
}

func machineRequestSetValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineRequestSet, _ ...state.CreateOption) error {
			return validateMachineRequestSet(ctx, st, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineRequestSet, newRes *omni.MachineRequestSet, _ ...state.UpdateOption) error {
			return validateMachineRequestSet(ctx, st, oldRes, newRes)
		})),
	}
}

func validateMachineRequestSet(ctx context.Context, st state.State, oldRes, res *omni.MachineRequestSet) error {
	if res.TypedSpec().Value.ProviderId == "" {
		return fmt.Errorf("provider id can not be empty")
	}

	if oldRes == nil || oldRes.TypedSpec().Value.ProviderData != res.TypedSpec().Value.ProviderData {
		if err := validateProviderData(ctx, st, res.TypedSpec().Value.ProviderId, res.TypedSpec().Value.ProviderData); err != nil {
			return err
		}
	}

	return validateTalosVersion(ctx, st, "", res.TypedSpec().Value.TalosVersion)
}

func validateTalosVersion(ctx context.Context, st state.State, current, newVersion string) error {
	var currentVersionIsDeprecated bool

	talosVersion, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(resources.DefaultNamespace, newVersion).Metadata())
	if err != nil {
		return fmt.Errorf("invalid talos version %q: %w", newVersion, err)
	}

	if current != "" {
		var ver *omni.TalosVersion

		ver, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(resources.DefaultNamespace, current).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if ver != nil {
			currentVersionIsDeprecated = ver.TypedSpec().Value.Deprecated
		}
	}

	// disallow updating to the deprecated Talos version from the non-deprecated one
	// 1.3.0 -> 1.3.7 should still work for example
	if talosVersion.TypedSpec().Value.Deprecated && !currentVersionIsDeprecated {
		return fmt.Errorf("talos version %q is no longer supported", newVersion)
	}

	return nil
}

func validateProviderData(ctx context.Context, st state.State, providerID, providerData string) error {
	validateSchema := func(providerStatus *infra.ProviderStatus) error {
		if providerStatus.TypedSpec().Value.Schema == "" {
			return nil
		}

		filename := fmt.Sprintf("%s-schema.json", providerStatus.Metadata().ID())

		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource(filename, strings.NewReader(providerStatus.TypedSpec().Value.Schema)); err != nil {
			return fmt.Errorf("failed to load json schema for provider %q: %w", providerID, err)
		}

		schema, err := compiler.Compile(filename)
		if err != nil {
			return fmt.Errorf("failed to load json schema for provider %q: %w", providerID, err)
		}

		// NaN type causes jsonschema validator to crash with nil reference error
		providerData = regexp.MustCompile(`(?i)\.nan`).ReplaceAllString(providerData, "null")

		var v interface{}
		if err = yaml.Unmarshal([]byte(providerData), &v); err != nil {
			return fmt.Errorf("failed to unmarshal provider data %w", err)
		}

		if v == nil {
			v = map[string]any{}
		}

		if err = schema.Validate(v); err != nil {
			return err
		}

		return nil
	}

	providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, st, providerID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	return validateSchema(providerStatus)
}

func infraMachineConfigValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes, newRes *omni.InfraMachineConfig, _ ...state.UpdateOption) error {
			if oldRes.TypedSpec().Value.Accepted && !newRes.TypedSpec().Value.Accepted {
				return errors.New("an accepted machine cannot be unaccepted")
			}

			return nil
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.InfraMachineConfig, _ ...state.DestroyOption) error {
			if !res.TypedSpec().Value.Accepted {
				return nil
			}

			if _, err := safe.StateGetByID[*siderolink.Link](ctx, st, res.Metadata().ID()); err != nil {
				if state.IsNotFoundError(err) {
					return nil
				}

				return err
			}

			return errors.New("cannot delete the config for an already accepted machine config while it is linked to a machine")
		})),
	}
}
