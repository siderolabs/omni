// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/image"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/auth/scope"
)

const (
	deprecatedControlPlaneRole = "role-controlplane"
	deprecatedWorkerRole       = "role-worker"
	deprecatedCluster          = "cluster"

	// ContractFixConfigPatch is the deprecated config patch introduced in 0.48.2 and dropped in 0.48.3.
	// we filter the patches with the following description to drop them in the migration.
	ContractFixConfigPatch = "Preserves the updated cluster features due to the version contract bug. " +
		"For more info, see: https://github.com/siderolabs/omni/issues/1095."
)

// populate Kubernetes versions and Talos versions ClusterMachineTemplates in the Cluster resources.
func clusterInfo(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.Cluster](ctx, s)
	if err != nil {
		return err
	}

	for val := range list.All() {
		item := val.TypedSpec()

		if item.Value.InstallImage == "" || item.Value.KubernetesVersion == "" { //nolint:staticcheck
			_, err = safe.StateUpdateWithConflicts(ctx, s, val.Metadata(), func(c *omni.Cluster) error {
				var items safe.List[*omni.ClusterMachineTemplate]

				items, err = safe.ReaderListAll[*omni.ClusterMachineTemplate](
					ctx,
					s,
					state.WithLabelQuery(resource.LabelEqual(deprecatedCluster, c.Metadata().ID())),
				)
				if err != nil {
					return err
				}

				if items.Len() == 0 {
					return nil
				}

				machine := items.Get(0)
				c.TypedSpec().Value.InstallImage = machine.TypedSpec().Value.InstallImage //nolint:staticcheck
				c.TypedSpec().Value.KubernetesVersion = machine.TypedSpec().Value.KubernetesVersion

				return nil
			})
			if err != nil {
				if state.IsPhaseConflictError(err) {
					continue
				}

				return err
			}
		}
	}

	return nil
}

func deprecateClusterMachineTemplates(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.ClusterMachineTemplate](ctx, s)
	if err != nil {
		return err
	}

	for val := range list.All() {
		item := val.TypedSpec()

		// generate user patch if it is defined
		if item.Value.Patch != "" {
			patch := omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("500-%s", uuid.New()),
				pair.MakePair("machine-uuid", val.Metadata().ID()),
			)

			helpers.CopyLabels(val, patch, deprecatedCluster)

			if err = createOrUpdate(ctx, s, patch, func(p *omni.ConfigPatch) error {
				if err = p.TypedSpec().Value.SetUncompressedData([]byte(item.Value.Patch)); err != nil {
					return err
				}

				return nil
			}, ""); err != nil {
				return err
			}
		}

		// generate install disk patch
		patch := omni.NewConfigPatch(
			resources.DefaultNamespace,
			fmt.Sprintf("000-%s", uuid.New()),
			pair.MakePair("machine-uuid", val.Metadata().ID()),
		)

		helpers.CopyLabels(val, patch, deprecatedCluster)

		if err = createOrUpdate(ctx, s, patch, func(p *omni.ConfigPatch) error {
			var config struct {
				MachineConfig struct {
					MachineInstall struct {
						InstallDisk string `yaml:"disk"`
					} `yaml:"install"`
				} `yaml:"machine"`
			}

			config.MachineConfig.MachineInstall.InstallDisk = item.Value.InstallDisk

			var data []byte
			data, err = yaml.Marshal(config)
			if err != nil {
				return err
			}

			if err = p.TypedSpec().Value.SetUncompressedData(data); err != nil {
				return err
			}

			p.Metadata().Labels().Set("machine-uuid", val.Metadata().ID())

			return nil
		}, ""); err != nil {
			return err
		}
	}

	return nil
}

func clusterMachinesToMachineSets(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.ClusterMachine](ctx, s)
	if err != nil {
		return err
	}

	machineSets := map[string]*omni.MachineSet{}

	// first gather all machines from the database and create backup data
	for item := range list.All() {
		labels := item.Metadata().Labels()

		clusterName, ok := labels.Get(deprecatedCluster)
		if !ok {
			logger.Warn("orphaned cluster machine", zap.String("id", item.Metadata().String()))

			continue
		}

		machineSetID := omni.WorkersResourceID(clusterName)

		if _, ok = item.Metadata().Labels().Get(deprecatedControlPlaneRole); ok {
			machineSetID = omni.ControlPlanesResourceID(clusterName)
		}

		if _, ok = machineSets[machineSetID]; !ok {
			machineSets[machineSetID] = omni.NewMachineSet(resources.DefaultNamespace, machineSetID)

			helpers.CopyLabels(item, machineSets[machineSetID], deprecatedCluster, deprecatedWorkerRole, deprecatedControlPlaneRole)
		}

		var patches []*omni.ConfigPatch

		patches, err = getConfigPatches(ctx, s, item, machineSets[machineSetID], "")
		if err != nil {
			return err
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, item.Metadata(), func(res *omni.ClusterMachine) error {
			res.Metadata().Labels().Set("machine-set", machineSetID)

			helpers.UpdateInputsVersions(res, patches...)

			owner := omnictrl.NewMachineSetController().ControllerName

			return res.Metadata().SetOwner(owner)
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(item.Metadata().Owner()))
		if err != nil {
			return err
		}

		machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, item.Metadata().ID(), machineSets[machineSetID])
		if err = createOrUpdate(ctx, s, machineSetNode, func(res *omni.MachineSetNode) error {
			helpers.CopyLabels(machineSetNode, res, deprecatedCluster, deprecatedWorkerRole)

			res.TypedSpec().Value = machineSetNode.TypedSpec().Value

			return nil
		}, "", state.WithExpectedPhaseAny()); err != nil {
			return err
		}
	}

	for _, ms := range machineSets {
		if err = createOrUpdate(ctx, s, ms, func(res *omni.MachineSet) error {
			helpers.CopyLabels(ms, res, deprecatedCluster, deprecatedWorkerRole)

			res.TypedSpec().Value = ms.TypedSpec().Value

			return nil
		}, "", state.WithExpectedPhaseAny()); err != nil {
			return err
		}
	}

	return nil
}

func changePublicKeyOwner(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	logger = logger.With(zap.String("migration", "changePublicKeyOwner"))

	list, err := safe.StateListAll[*auth.PublicKey](ctx, s)
	if err != nil {
		return err
	}

	for item := range list.All() {
		if item.Metadata().Owner() != "" {
			continue
		}

		logger.Info("updating public key with empty owner", zap.String("id", item.Metadata().String()))

		_, err = safe.StateUpdateWithConflicts(ctx, s, item.Metadata(), func(res *auth.PublicKey) error {
			return res.Metadata().SetOwner((&omnictrl.KeyPrunerController{}).Name())
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(item.Metadata().Owner()))
		if err != nil {
			return err
		}
	}

	return nil
}

func addDefaultScopesToUsers(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	logger = logger.With(zap.String("migration", "addDefaultScopesToUsers"))

	list, err := safe.StateListAll[*auth.User](ctx, s)
	if err != nil {
		return err
	}

	scopes := scope.NewScopes(scope.UserDefaultScopes...).Strings()

	for user := range list.All() {
		if len(user.TypedSpec().Value.GetScopes()) == 0 {
			logger.Info("adding scopes to user",
				zap.String("id", user.Metadata().String()), zap.Strings("scopes", scopes),
			)

			_, err = safe.StateUpdateWithConflicts(ctx, s, user.Metadata(), func(u *auth.User) error {
				u.TypedSpec().Value.Scopes = scopes //nolint:staticcheck

				return nil
			}, state.WithExpectedPhaseAny())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func setRollingStrategyOnControlPlaneMachineSets(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	logger = logger.With(zap.String("migration", "setRollingStrategyOnControlPlaneMachineSets"))

	list, err := safe.StateListAll[*omni.MachineSet](
		ctx,
		s,
		state.WithLabelQuery(resource.LabelExists(deprecatedControlPlaneRole)),
	)
	if err != nil {
		return err
	}

	for machineSet := range list.All() {
		if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		if machineSet.TypedSpec().Value.GetUpdateStrategy() == specs.MachineSetSpec_Unset {
			logger.Info("setting rolling update strategy on control plane machine set",
				zap.String("id", machineSet.Metadata().String()),
			)

			_, err = safe.StateUpdateWithConflicts(ctx, s, machineSet.Metadata(), func(ms *omni.MachineSet) error {
				ms.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func updateConfigPatchLabels(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](
		ctx,
		s,
	)
	if err != nil {
		return err
	}

	clusterMachineToCluster := map[resource.ID]resource.ID{}

	for clusterMachine := range clusterMachineList.All() {
		cluster, _ := clusterMachine.Metadata().Labels().Get(deprecatedCluster)
		clusterMachineToCluster[clusterMachine.Metadata().ID()] = cluster
	}

	machineSetList, err := safe.StateListAll[*omni.MachineSet](ctx, s)
	if err != nil {
		return err
	}

	machineSetToCluster := map[resource.ID]resource.ID{}

	for machineSet := range machineSetList.All() {
		cluster, _ := machineSet.Metadata().Labels().Get(deprecatedCluster)
		machineSetToCluster[machineSet.Metadata().ID()] = cluster
	}

	configPatchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, s)
	if err != nil {
		return err
	}

	for val := range configPatchList.All() {
		_, err = safe.StateUpdateWithConflicts(ctx, s, val.Metadata(), func(patch *omni.ConfigPatch) error {
			labels := patch.Metadata().Labels()

			clusterName, clusterNameOk := labels.Get("cluster-name")
			if clusterNameOk {
				labels.Set(deprecatedCluster, clusterName)
				labels.Delete("cluster-name")
			}

			machineSet, machineSetOk := labels.Get("machine-set-name")
			if machineSetOk {
				labels.Set("machine-set", machineSet)
				labels.Delete("machine-set-name")

				cluster := machineSetToCluster[machineSet]
				if cluster != "" {
					labels.Set(deprecatedCluster, cluster)
				}
			}

			clusterMachine, clusterMachineOk := labels.Get("machine-uuid")
			if clusterMachineOk {
				labels.Set("cluster-machine", clusterMachine)
				labels.Delete("machine-uuid")

				cluster := clusterMachineToCluster[clusterMachine]
				if cluster != "" {
					labels.Set(deprecatedCluster, cluster)
				}
			}

			return nil
		}, state.WithExpectedPhaseAny())
		if err != nil {
			return err
		}
	}

	return nil
}

// migrate finalizer on the Machine from 'ClusterMachineStatusController' to 'MachineSetStatusController'.
func updateMachineFinalizers(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	logger = logger.With(zap.String("migration", "updateMachineFinalizers"))

	machineList, err := safe.StateListAll[*omni.Machine](ctx, s)
	if err != nil {
		return err
	}

	for machine := range machineList.All() {
		if machine.Metadata().Finalizers().Add("ClusterMachineStatusController") {
			// no finalizer, skip it
			continue
		}

		cms, err := safe.StateGet[*omni.ClusterMachine](ctx, s, omni.NewClusterMachine(resources.DefaultNamespace, machine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if err = s.RemoveFinalizer(ctx, machine.Metadata(), "ClusterMachineStatusController"); err != nil {
			return err
		}

		if cms != nil {
			if err = s.AddFinalizer(ctx, machine.Metadata(), "MachineSetStatusController"); err != nil {
				return err
			}
		}

		logger.Info("updated machine finalizers", zap.String("machine", machine.Metadata().ID()))
	}

	return nil
}

func labelConfigPatches(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	patchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, s)
	if err != nil {
		return err
	}

	for patch := range patchList.All() {
		// if the patch is prefixed with 000- then it's install disk patch and it should be updated with appropriate labels
		if strings.HasPrefix(patch.Metadata().ID(), "000-") {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, patch.Metadata(), func(res *omni.ConfigPatch) error {
				res.Metadata().Labels().Set("system-patch", "")

				return nil
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func updateMachineStatusClusterRelations(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	msList, err := safe.StateListAll[*omni.MachineStatus](ctx, s)
	if err != nil {
		return err
	}

	for ms := range msList.All() {
		if ms.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, ms.Metadata(), func(res *omni.MachineStatus) error {
			labels := res.Metadata().Labels()
			spec := res.TypedSpec().Value

			cluster, ok := labels.Get(deprecatedCluster)
			if !ok {
				cluster = spec.Cluster
			}

			if cluster == "" {
				labels.Delete(deprecatedControlPlaneRole)
				labels.Delete(deprecatedWorkerRole)

				labels.Set("available", "")

				return nil
			}

			spec.Cluster = cluster

			_, controlPlane := res.Metadata().Labels().Get(deprecatedControlPlaneRole)
			if controlPlane {
				spec.Role = specs.MachineStatusSpec_CONTROL_PLANE
			}

			_, worker := labels.Get(deprecatedWorkerRole)
			if worker {
				spec.Role = specs.MachineStatusSpec_WORKER
			}

			labels.Delete(deprecatedCluster)
			labels.Delete(deprecatedControlPlaneRole)
			labels.Delete(deprecatedWorkerRole)

			return nil
		}, state.WithUpdateOwner(ms.Metadata().Owner()))
		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:staticcheck
func addServiceAccountScopesToUsers(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	userList, err := safe.StateListAll[*auth.User](ctx, s)
	if err != nil {
		return err
	}

	for user := range userList.All() {
		_, err = safe.StateUpdateWithConflicts(ctx, s, user.Metadata(), func(u *auth.User) error {
			if !slices.ContainsFunc(u.TypedSpec().Value.Scopes, func(s string) bool {
				return s == scope.ServiceAccountAny.String()
			}) {
				u.TypedSpec().Value.Scopes = append(u.TypedSpec().Value.Scopes, scope.ServiceAccountAny.String())
			}

			return nil
		}, state.WithUpdateOwner(user.Metadata().Owner()))
		if err != nil {
			return err
		}
	}

	publicKeyList, err := safe.StateListAll[*auth.PublicKey](ctx, s)
	if err != nil {
		return err
	}

	for key := range publicKeyList.All() {
		_, err = safe.StateUpdateWithConflicts(ctx, s, key.Metadata(), func(pk *auth.PublicKey) error {
			pk.TypedSpec().Value.Scopes = append(pk.TypedSpec().Value.Scopes, scope.ServiceAccountAny.String())

			return nil
		}, state.WithUpdateOwner(key.Metadata().Owner()))
		if err != nil {
			return err
		}
	}

	return nil
}

func clusterInstallImageToTalosVersion(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	clusterList, err := safe.StateListAll[*omni.Cluster](ctx, s)
	if err != nil {
		return err
	}

	for cluster := range clusterList.All() {
		if cluster.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, cluster.Metadata(), func(res *omni.Cluster) error {
			var version string

			version, err = image.GetTag(res.TypedSpec().Value.InstallImage) //nolint:staticcheck
			if err != nil {
				return fmt.Errorf("failed to migrate %s to the new scheme, install image %q: %w", cluster.Metadata(), res.TypedSpec().Value.InstallImage, err) //nolint:staticcheck
			}

			res.TypedSpec().Value.TalosVersion = strings.TrimLeft(version, "v")

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateLabels(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	for _, r := range registry.Resources {
		definition := r.ResourceDefinition()

		if definition.DefaultNamespace != resources.DefaultNamespace ||
			definition.Type == omni.MachineLabelsType ||
			definition.Type == auth.IdentityType ||
			definition.Type == auth.UserType {
			continue
		}

		list, err := s.List(ctx, resource.NewMetadata(definition.DefaultNamespace, definition.Type, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		for _, r := range list.Items {
			_, err = s.UpdateWithConflicts(ctx, r.Metadata(), func(res resource.Resource) error {
				for label, value := range res.Metadata().Labels().Raw() {
					if !strings.HasPrefix(label, omni.SystemLabelPrefix) {
						res.Metadata().Labels().Set(omni.SystemLabelPrefix+label, value)
					}
				}

				return nil
			}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(r.Metadata().Owner()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func dropOldLabels(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	dropLabels := map[string]struct{}{
		"address":                   {},
		"arch":                      {},
		"available":                 {},
		"cluster":                   {},
		"cluster-machine":           {},
		"connected":                 {},
		"cores":                     {},
		"cpu":                       {},
		"disconnected":              {},
		"hostname":                  {},
		"instance":                  {},
		"machine":                   {},
		"machine-set":               {},
		"machine-set-skip-teardown": {},
		"mem":                       {},
		"net":                       {},
		"node-name":                 {},
		"platform":                  {},
		"region":                    {},
		"reporting-events":          {},
		"role-controlplane":         {},
		"role-worker":               {},
		"storage":                   {},
		"system-patch":              {},
		"zone":                      {},
	}

	for _, r := range registry.Resources {
		definition := r.ResourceDefinition()

		if definition.DefaultNamespace != resources.DefaultNamespace ||
			definition.Type == omni.MachineLabelsType {
			continue
		}

		list, err := s.List(ctx, resource.NewMetadata(definition.DefaultNamespace, definition.Type, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		for _, r := range list.Items {
			_, err = s.UpdateWithConflicts(ctx, r.Metadata(), func(res resource.Resource) error {
				res.Metadata().Labels().Do(func(kvutils.TempKV) {
					for label, value := range res.Metadata().Labels().Raw() {
						_, ok := dropLabels[label]
						if !ok {
							continue
						}

						copiedValue, exists := res.Metadata().Labels().Get(omni.SystemLabelPrefix + label)
						if !exists || copiedValue != value {
							continue
						}

						res.Metadata().Labels().Delete(label)
					}
				})

				return nil
			}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(r.Metadata().Owner()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func convertScopesToRoles(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	scopesToRole := func(scopes scope.Scopes) role.Role {
		scopeList := scopes.List()
		if len(scopeList) == 0 {
			return role.None
		}

		readOnly := true

		// a scope with any kind of access to the users or to the service accounts implies being an admin
		for _, s := range scopeList {
			if s.Object == scope.ObjectUser || s.Object == scope.ObjectServiceAccount {
				return role.Admin
			}

			if s.Action != scope.ActionRead {
				readOnly = false
			}
		}

		if readOnly {
			return role.Reader
		}

		return role.Operator
	}

	// users
	userList, err := safe.StateListAll[*auth.User](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for item := range userList.All() {
		if _, err = safe.StateUpdateWithConflicts(ctx, st, item.Metadata(), func(user *auth.User) error {
			scopes, scopesErr := scope.ParseScopes(user.TypedSpec().Value.GetScopes())
			if scopesErr != nil {
				return fmt.Errorf("failed to parse scopes: %w", scopesErr)
			}

			user.TypedSpec().Value.Role = string(scopesToRole(scopes))

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(item.Metadata().Owner())); err != nil {
			return fmt.Errorf("failed to update user %q: %w", item.Metadata(), err)
		}
	}

	// public keys
	pubKeyList, err := safe.StateListAll[*auth.PublicKey](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list public keys: %w", err)
	}

	for item := range pubKeyList.All() {
		if _, err = safe.StateUpdateWithConflicts(ctx, st, item.Metadata(), func(pubKey *auth.PublicKey) error {
			scopes, scopesErr := scope.ParseScopes(pubKey.TypedSpec().Value.GetScopes())
			if scopesErr != nil {
				return fmt.Errorf("failed to parse scopes: %w", scopesErr)
			}

			pubKey.TypedSpec().Value.Role = string(scopesToRole(scopes))

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(item.Metadata().Owner())); err != nil {
			return fmt.Errorf("failed to update public key %q: %w", item.Metadata(), err)
		}
	}

	return nil
}

func lowercaseAllIdentities(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	identities, err := safe.StateListAll[*auth.Identity](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	migrated := map[string]struct{}{}

	if err = identities.ForEachErr(func(identity *auth.Identity) error {
		id := identity.Metadata().ID()
		lowercase := strings.ToLower(id)

		if id == lowercase {
			return nil
		}

		var existing *auth.Identity

		existing, err = safe.ReaderGet[*auth.Identity](ctx, st, auth.NewIdentity(identity.Metadata().Namespace(), lowercase).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		switch {
		case existing == nil:
			res := auth.NewIdentity(identity.Metadata().Namespace(), lowercase)
			helpers.CopyAllLabels(identity, res)

			res.TypedSpec().Value = identity.TypedSpec().Value

			if err = st.Create(ctx, res); err != nil {
				return err
			}
		case existing.Metadata().Created().After(identity.Metadata().Created()):
		default:
			if _, err = safe.StateUpdateWithConflicts(ctx, st, existing.Metadata(), func(res *auth.Identity) error {
				helpers.CopyAllLabels(identity, res)

				res.TypedSpec().Value = identity.TypedSpec().Value

				return nil
			}); err != nil {
				return err
			}
		}

		if err = st.Destroy(ctx, identity.Metadata()); err != nil {
			return err
		}

		migrated[id] = struct{}{}

		return nil
	}); err != nil {
		return err
	}

	if len(migrated) == 0 {
		return nil
	}

	var keys safe.List[*auth.PublicKey]

	keys, err = safe.StateListAll[*auth.PublicKey](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	return keys.ForEachErr(func(k *auth.PublicKey) error {
		if k.TypedSpec().Value.Identity == nil {
			return nil
		}

		if _, ok := migrated[k.TypedSpec().Value.Identity.Email]; !ok {
			return nil
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, k.Metadata(), func(res *auth.PublicKey) error {
			res.TypedSpec().Value.Identity.Email = strings.ToLower(res.TypedSpec().Value.Identity.Email)

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner((&omnictrl.KeyPrunerController{}).Name()))

		return err
	})
}

func removeConfigPatchesFromClusterMachines(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	items, err := safe.ReaderListAll[*omni.ClusterMachine](
		ctx,
		st,
	)
	if err != nil {
		return err
	}

	getPatchData := func(p *omni.ConfigPatch) (string, error) {
		buffer, bufferErr := p.TypedSpec().Value.GetUncompressedData()
		if bufferErr != nil {
			return "", err
		}

		defer buffer.Free()

		data := buffer.Data()

		return string(data), err
	}

	return items.ForEachErr(func(item *omni.ClusterMachine) error {
		owner := omnictrl.NewMachineSetController().ControllerName

		err = createOrUpdate(ctx, st, omni.NewClusterMachineConfigPatches(item.Metadata().Namespace(), item.Metadata().ID()),
			func(res *omni.ClusterMachineConfigPatches) error {
				helpers.CopyAllLabels(item, res)

				machineSet, ok := item.Metadata().Labels().Get(omni.SystemLabelPrefix + "machine-set")
				if !ok {
					return nil
				}

				var patches []*omni.ConfigPatch

				patches, err = getConfigPatches(ctx, st, item, omni.NewMachineSet(resources.DefaultNamespace, machineSet), omni.SystemLabelPrefix)
				if err != nil {
					return err
				}

				patchesRaw := make([]string, 0, len(patches))

				for _, p := range patches {
					data, dataErr := getPatchData(p)
					if dataErr != nil {
						return dataErr
					}

					patchesRaw = append(patchesRaw, data)
				}

				return res.TypedSpec().Value.SetUncompressedPatches(patchesRaw)
			}, owner, state.WithExpectedPhaseAny(),
		)
		if err != nil {
			return err
		}

		// cleanup patches data from the resource to free up space
		if err = createOrUpdate(ctx, st, item, func(*omni.ClusterMachine) error {
			return nil
		}, owner, state.WithExpectedPhaseAny()); err != nil {
			return err
		}

		return reconcileConfigInputs(ctx, st, item, false, false)
	})
}

// this migration does the following:
//   - create machine config gen options resources.
//   - reconcile machine config version to avoid triggering config apply calls on all machines.
func machineInstallDiskPatches(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	items, err := safe.ReaderListAll[*omni.ClusterMachine](
		ctx,
		st,
	)
	if err != nil {
		return err
	}

	return items.ForEachErr(func(item *omni.ClusterMachine) error {
		machineStatus, err := safe.StateGet[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, item.Metadata().ID()).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		if err = createOrUpdate(ctx, st, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, item.Metadata().ID()), func(res *omni.MachineConfigGenOptions) error {
			omnictrl.GenInstallConfig(machineStatus, nil, res)

			return nil
		}, omnictrl.MachineConfigGenOptionsControllerName); err != nil {
			return err
		}

		return reconcileConfigInputs(ctx, st, item, true, false)
	})
}

// siderolinkCounters moves data from siderolink.DeprecatedLinkCounter resource to omni.MachineStatusLink resource.
//
// The rest of MachineStatusLink resource will be filled by the controller.
// siderolink.DeprecatedLinkCounter will be destroyed at the end of the migration.
func siderolinkCounters(ctx context.Context, s state.State, logger *zap.Logger, _ migrationContext) error {
	logger = logger.With(zap.String("migration", "siderolinkCounters"))

	list, err := safe.StateListAll[*siderolink.DeprecatedLinkCounter](ctx, s)
	if err != nil {
		return err
	}

	migrated := 0

	for linkCounter := range list.All() {
		if err = createOrUpdate(ctx, s, omni.NewMachineStatusLink(resources.MetricsNamespace, linkCounter.Metadata().ID()),
			func(res *omni.MachineStatusLink) error {
				res.TypedSpec().Value.SiderolinkCounter = linkCounter.TypedSpec().Value

				return nil
			}, omnictrl.NewMachineStatusLinkController(nil).Name()); err != nil {
			return err
		}

		if err = s.Destroy(ctx, linkCounter.Metadata(), state.WithDestroyOwner(linkCounter.Metadata().Owner())); err != nil {
			return err
		}

		migrated++
	}

	logger.Info("migrated siderolink counters", zap.Int("count", migrated))

	return nil
}

// this migration fixes ownership of all ClusterConfigVersion resources, it was initially created by ClusterController,
// but then it got it's own ClusterConfigVersion controller.
func fixClusterTalosVersionOwnership(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.ClusterConfigVersion](ctx, s)
	if err != nil {
		return err
	}

	expectedOwner := omnictrl.NewClusterConfigVersionController().ControllerName

	return list.ForEachErr(func(r *omni.ClusterConfigVersion) error {
		owner := r.Metadata().Owner()
		if owner == expectedOwner {
			return nil
		}

		updated := omni.NewClusterConfigVersion(resources.DefaultNamespace, r.Metadata().ID())
		updated.TypedSpec().Value = r.TypedSpec().Value
		updated.Metadata().SetVersion(r.Metadata().Version())

		if err = updated.Metadata().SetOwner(expectedOwner); err != nil {
			return err
		}

		return s.Update(ctx, updated, state.WithUpdateOwner(owner))
	})
}

// add machine-set label to all cluster machine config patches resources.
func updateClusterMachineConfigPatchesLabels(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.ClusterMachineConfigPatches](ctx, s)
	if err != nil {
		return err
	}

	return list.ForEachErr(func(res *omni.ClusterMachineConfigPatches) error {
		clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, s, res.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.ClusterMachineConfigPatches) error {
			helpers.CopyAllLabels(clusterMachine, r)

			return nil
		}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())

		return err
	})
}

// clearEmptyConfigPatches removes empty patches from all ClusterMachineConfigPatches resources.
func clearEmptyConfigPatches(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.ClusterMachineConfigPatches](ctx, s)

	return list.ForEachErr(func(res *omni.ClusterMachineConfigPatches) error {
		patches, getErr := res.TypedSpec().Value.GetUncompressedPatches()
		if getErr != nil {
			return getErr
		}

		if len(patches) == 0 {
			return nil
		}

		var filteredPatches []string

		for _, patch := range patches {
			if strings.TrimSpace(patch) != "" {
				filteredPatches = append(filteredPatches, patch)
			}
		}

		if len(filteredPatches) == len(patches) {
			return nil
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.ClusterMachineConfigPatches) error {
			return r.TypedSpec().Value.SetUncompressedPatches(filteredPatches)
		}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())

		return err
	})
}

// removes schematic configurations which are not associated with the machines.
func cleanupDanglingSchematicConfigurations(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.SchematicConfiguration](ctx, s)
	if err != nil {
		return err
	}

	return list.ForEachErr(func(res *omni.SchematicConfiguration) error {
		_, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, s, res.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				// remove finalizers and destroy the resource
				_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.SchematicConfiguration) error {
					for _, f := range *r.Metadata().Finalizers() {
						r.Metadata().Finalizers().Remove(f)
					}

					return nil
				}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())
				if err != nil {
					return err
				}

				return s.Destroy(ctx, res.Metadata(), state.WithDestroyOwner(res.Metadata().Owner()))
			}

			return err
		}

		return nil
	})
}

func cleanupExtensionsConfigurationStatuses(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.ExtensionsConfigurationStatus](ctx, s)
	if err != nil {
		return err
	}

	return list.ForEachErr(func(res *omni.ExtensionsConfigurationStatus) error {
		// remove finalizers and destroy the resource
		_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.ExtensionsConfigurationStatus) error {
			for _, f := range *r.Metadata().Finalizers() {
				r.Metadata().Finalizers().Remove(f)
			}

			return nil
		}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())
		if err != nil {
			return err
		}

		return s.Destroy(ctx, res.Metadata(), state.WithDestroyOwner(res.Metadata().Owner()))
	})
}

func dropSchematicConfigurationsControllerFinalizer(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.ExtensionsConfiguration](ctx, s)
	if err != nil {
		return err
	}

	return list.ForEachErr(func(res *omni.ExtensionsConfiguration) error {
		if !res.Metadata().Finalizers().Has(omnictrl.SchematicConfigurationControllerName) {
			return nil
		}

		// remove finalizers and destroy the resource
		_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.ExtensionsConfiguration) error {
			r.Metadata().Finalizers().Remove(omnictrl.SchematicConfigurationControllerName)

			return nil
		}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())

		return err
	})
}

// generateAllMaintenanceConfigs reconciles maintenance configs for all machines and update the inputs to avoid triggering config updates for each machine.
func generateAllMaintenanceConfigs(context.Context, state.State, *zap.Logger, migrationContext) error {
	// deprecated
	return nil
}

// setMachineStatusSnapshotOwner reconciles maintenance configs for all machines and update the inputs to avoid triggering config updates for each machine.
func setMachineStatusSnapshotOwner(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.MachineStatusSnapshot](ctx, st)
	if err != nil {
		return err
	}

	for item := range list.All() {
		if item.Metadata().Owner() != "" {
			continue
		}

		logger.Info("updating machine status snapshot with empty owner", zap.String("id", item.Metadata().String()))

		_, err = safe.StateUpdateWithConflicts(ctx, st, item.Metadata(), func(res *omni.MachineStatusSnapshot) error {
			return res.Metadata().SetOwner(omnictrl.MachineStatusSnapshotControllerName)
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(item.Metadata().Owner()))
		if err != nil {
			return err
		}
	}

	return nil
}

// migrateInstallImageConfigIntoGenOptions creates the initial InstallImage resources using the existing MachineStatus and ClusterMachineTalosVersion resources.
//
// It then reconciles the config inputs for all machines to avoid triggering config updates for each machine.
func migrateInstallImageConfigIntoGenOptions(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.StateListAll[*omni.MachineConfigGenOptions](ctx, st)
	if err != nil {
		return err
	}

	for genOptions := range list.All() {
		if genOptions.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		var (
			machineStatus *omni.MachineStatus
			talosVersion  *omni.ClusterMachineTalosVersion
		)

		if machineStatus, err = safe.StateGet[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, genOptions.Metadata().ID()).Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if talosVersion, err = safe.StateGet[*omni.ClusterMachineTalosVersion](ctx, st,
			omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, genOptions.Metadata().ID()).Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if _, err = safe.StateUpdateWithConflicts(ctx, st, genOptions.Metadata(), func(res *omni.MachineConfigGenOptions) error {
			omnictrl.GenInstallConfig(machineStatus, talosVersion, res)

			return nil
		}, state.WithUpdateOwner(genOptions.Metadata().Owner())); err != nil {
			return err
		}

		var clusterMachine *omni.ClusterMachine

		if clusterMachine, err = safe.StateGetByID[*omni.ClusterMachine](ctx, st, genOptions.Metadata().ID()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		// ClusterMachineTalosVersion is no more used as an input in the hash, exclude it
		if err = reconcileConfigInputs(ctx, st, clusterMachine, true, false); err != nil {
			return err
		}
	}

	return nil
}

// dropGeneratedMaintenanceConfigs drops all generated maintenance configs, they will be generated by the controller.
func dropGeneratedMaintenanceConfigs(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	items, err := safe.ReaderListAll[*omni.MachineStatus](
		ctx,
		st,
	)
	if err != nil {
		return err
	}

	return items.ForEachErr(func(item *omni.MachineStatus) error {
		configPatch, err := safe.ReaderGetByID[*omni.ConfigPatch](ctx, st, MaintenanceConfigPatchPrefix+item.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		// remove finalizers and destroy the resource
		_, err = safe.StateUpdateWithConflicts(ctx, st, configPatch.Metadata(), func(r *omni.ConfigPatch) error {
			for _, f := range *r.Metadata().Finalizers() {
				r.Metadata().Finalizers().Remove(f)
			}

			return nil
		}, state.WithUpdateOwner(configPatch.Metadata().Owner()), state.WithExpectedPhaseAny())
		if err != nil {
			return err
		}

		if err = st.Destroy(ctx, configPatch.Metadata(), state.WithDestroyOwner(configPatch.Metadata().Owner())); err != nil {
			return err
		}

		if err = st.RemoveFinalizer(ctx, item.Metadata(), "MaintenanceConfigPatchController"); err != nil && !state.IsPhaseConflictError(err) {
			return err
		}

		return nil
	})
}

func deleteAllResources(md resource.Metadata) func(context.Context, state.State, *zap.Logger, migrationContext) error {
	return func(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
		list, err := st.List(ctx, md)
		if err != nil {
			return err
		}

		for _, r := range list.Items {
			_, err = st.UpdateWithConflicts(ctx, r.Metadata(), func(res resource.Resource) error {
				for _, f := range *res.Metadata().Finalizers() {
					res.Metadata().Finalizers().Remove(f)
				}

				return nil
			}, state.WithUpdateOwner(r.Metadata().Owner()), state.WithExpectedPhaseAny())
			if err != nil {
				return err
			}

			if err = st.Destroy(ctx, r.Metadata(), state.WithDestroyOwner(r.Metadata().Owner())); err != nil {
				return err
			}
		}

		return nil
	}
}

func removeMaintenanceConfigPatchFinalizers(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	items, err := safe.ReaderListAll[*omni.MachineStatus](
		ctx,
		st,
	)
	if err != nil {
		return err
	}

	return items.ForEachErr(func(item *omni.MachineStatus) error {
		return st.RemoveFinalizer(ctx, item.Metadata(), "MaintenanceConfigPatchController")
	})
}

func noopMigration(context.Context, state.State, *zap.Logger, migrationContext) error { return nil }

func compressConfigPatches(ctx context.Context, st state.State, l *zap.Logger, _ migrationContext) error {
	doConfigPatch := updateSingle[string, specs.ConfigPatchSpec, *specs.ConfigPatchSpec](2048)

	for _, fn := range []func(context.Context, state.State, *zap.Logger) error{
		compressUncompressed[*omni.ConfigPatch](doConfigPatch),
	} {
		if err := fn(ctx, st, l); err != nil {
			return err
		}
	}

	return nil
}

func compressUncompressed[
	R interface {
		generic.ResourceWithRD
		TypedSpec() *protobuf.ResourceSpec[T, S]
	},
	T any,
	S protobuf.Spec[T],
](update func(spec *protobuf.ResourceSpec[T, S]) (updateResult, error)) func(context.Context, state.State, *zap.Logger) error {
	return func(ctx context.Context, st state.State, l *zap.Logger) error {
		items, err := safe.ReaderListAll[R](ctx, st)
		if err != nil {
			return fmt.Errorf("failed to list resources for compression: %w", err)
		}

		if total := items.Len(); total > 0 {
			md := items.Get(0).Metadata()

			l = l.With(zap.String("resource", md.String()), zap.Int("total_num", total))

			l.Info("compressing resources")
		}

		processed := 0
		alreadyCompressed := 0
		belowThresholdNum := 0
		nonRunning := 0

		for val := range items.All() {
			if val.Metadata().Phase() != resource.PhaseRunning {
				nonRunning++

				continue
			}

			spec := val.TypedSpec()

			result, uerr := update(spec)
			if uerr != nil {
				return fmt.Errorf("failed to compress %s: %w", val.Metadata().ID(), uerr)
			}

			switch result {
			case emptyData:
				alreadyCompressed++

				continue
			case belowThreshold:
				belowThresholdNum++

				continue
			case compressed:
				processed++
			}

			if _, err = safe.StateUpdateWithConflicts(ctx, st, val.Metadata(), func(res R) error {
				res.TypedSpec().Value = spec.Value

				return nil
			}, state.WithUpdateOwner(val.Metadata().Owner())); err != nil {
				return fmt.Errorf("failed to update %s: %w", val.Metadata(), err)
			}
		}

		l.Info("compress migration done",
			zap.Int("compressed", processed),
			zap.Int("skipped_already_compressed", alreadyCompressed),
			zap.Int("skipped_non_running", nonRunning),
			zap.Int("skipped_below_threshold", belowThresholdNum),
		)

		return nil
	}
}

func updateSingle[
	D string | []byte,
	T any,
	S interface {
		GetData() D
		SetUncompressedData(data []byte, opts ...specs.CompressionOption) error
		protobuf.Spec[T]
	},
](threshold int) func(spec *protobuf.ResourceSpec[T, S]) (updateResult, error) {
	return func(spec *protobuf.ResourceSpec[T, S]) (updateResult, error) {
		data := spec.Value.GetData()

		switch size := len(data); {
		case size == 0:
			return emptyData, nil
		case size < threshold:
			return belowThreshold, nil
		}

		err := spec.Value.SetUncompressedData([]byte(data))
		if err != nil {
			return emptyData, fmt.Errorf("failed to compress data during migration: %w", err)
		}

		return compressed, nil
	}
}

type updateResult int

const (
	emptyData updateResult = iota + 1
	belowThreshold
	compressed
)

func moveEtcdBackupStatuses(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	statusOldNamespaceMD := resource.NewMetadata(resources.DefaultNamespace, omni.EtcdBackupStatusType, "", resource.VersionUndefined)

	statusList, err := safe.StateList[*omni.EtcdBackupStatus](ctx, st, statusOldNamespaceMD)
	if err != nil {
		return fmt.Errorf("failed to list etcd backup statuses: %w", err)
	}

	numMigrated := 0

	for oldStatus := range statusList.All() {
		if oldStatus.Metadata().Phase() == resource.PhaseRunning {
			newStatus := omni.NewEtcdBackupStatus(oldStatus.Metadata().ID())

			if err = newStatus.Metadata().SetOwner(oldStatus.Metadata().Owner()); err != nil {
				return fmt.Errorf("failed to set owner for %q: %w", newStatus.Metadata(), err)
			}

			// No labels, annotations, etc. to copy

			newStatus.TypedSpec().Value = oldStatus.TypedSpec().Value

			if err = st.Create(ctx, newStatus, state.WithCreateOwner(oldStatus.Metadata().Owner())); err != nil {
				return fmt.Errorf("failed to create %q: %w", newStatus.Metadata(), err)
			}

			numMigrated++
		}

		// This resource does not contain finalizers, so it's safe to destroy
		if err = st.Destroy(ctx, oldStatus.Metadata(), state.WithDestroyOwner(oldStatus.Metadata().Owner())); err != nil {
			return fmt.Errorf("failed to destroy %q: %w", oldStatus.Metadata(), err)
		}
	}

	logger.Info("migrated etcd backup statuses", zap.Int("num_migrated", numMigrated), zap.Int("num_total", statusList.Len()))

	overallStatusOldNamespaceMD := resource.NewMetadata(resources.DefaultNamespace, omni.EtcdBackupOverallStatusType, omni.EtcdBackupOverallStatusID, resource.VersionUndefined)

	oldOverallStatus, err := safe.StateGet[*omni.EtcdBackupOverallStatus](ctx, st, overallStatusOldNamespaceMD)
	if err != nil {
		if state.IsNotFoundError(err) {
			logger.Info("etcd backup overall status not found, skipping...")

			return nil
		}

		return fmt.Errorf("failed to get etcd backup overall status: %w", err)
	}

	if oldOverallStatus.Metadata().Phase() == resource.PhaseRunning {
		newOverallStatus := omni.NewEtcdBackupOverallStatus()

		if err = newOverallStatus.Metadata().SetOwner(oldOverallStatus.Metadata().Owner()); err != nil {
			return fmt.Errorf("failed to set owner for %q: %w", newOverallStatus.Metadata(), err)
		}

		// No labels, annotations, etc. to copy

		newOverallStatus.TypedSpec().Value = oldOverallStatus.TypedSpec().Value

		if err = st.Create(ctx, newOverallStatus, state.WithCreateOwner(oldOverallStatus.Metadata().Owner())); err != nil {
			return fmt.Errorf("failed to create %q: %w", newOverallStatus.Metadata(), err)
		}
	}

	// This resource does not contain finalizers, so it's safe to destroy
	if err = st.Destroy(ctx, oldOverallStatus.Metadata(), state.WithDestroyOwner(oldOverallStatus.Metadata().Owner())); err != nil {
		return fmt.Errorf("failed to destroy %q: %w", oldOverallStatus.Metadata(), err)
	}

	logger.Info("migrate etcd backup overall status", zap.Bool("created", oldOverallStatus.Metadata().Phase() == resource.PhaseRunning))

	return nil
}

func dropObsoleteConfigPatches(ctx context.Context, st state.State, logger *zap.Logger, migrationContext migrationContext) error {
	patchMigrationIndex := slices.IndexFunc(migrationContext.migrations, func(m *migration) bool {
		return m.name == "oldVersionContractFix"
	})

	if patchMigrationIndex == -1 {
		return fmt.Errorf("failed to find the old version contract fix migration")
	}

	if migrationContext.initialDBVersion <= uint64(patchMigrationIndex) {
		logger.Info("nothing to do because of the db version", zap.Uint64("initial_db_version", migrationContext.initialDBVersion), zap.Int("expect_greater", patchMigrationIndex))

		return nil
	}

	configPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, st,
		state.WithLabelQuery(resource.LabelExists(omni.LabelCluster)),
		state.WithLabelQuery(resource.LabelExists(omni.LabelClusterMachine)),
	)
	if err != nil {
		return err
	}

	for r := range configPatches.All() {
		description, ok := r.Metadata().Annotations().Get(omni.ConfigPatchDescription)
		if !ok {
			continue
		}

		if description != ContractFixConfigPatch {
			continue
		}

		clusterMachine, ok := r.Metadata().Labels().Get(omni.LabelClusterMachine)
		if !ok {
			continue
		}

		logger.Info("removing obsolete config patch",
			zap.String("patch_id", r.Metadata().ID()),
			zap.String("id", clusterMachine),
		)

		_, err = safe.StateUpdateWithConflicts(ctx, st, r.Metadata(), func(res *omni.ConfigPatch) error {
			for _, f := range *res.Metadata().Finalizers() {
				res.Metadata().Finalizers().Remove(f)
			}

			return nil
		}, state.WithExpectedPhaseAny())
		if err != nil {
			return err
		}

		if err = st.Destroy(ctx, r.Metadata()); err != nil {
			return err
		}
	}

	return nil
}

// markVersionContract marks the cluster machine as being affected by the issue:
// https://github.com/siderolabs/omni/issues/1097
//
// We create a system config patch with a low weight (so that the user patches will override it)
// to preserve the config changes the revert would cause which would require a reboot, namely:
// - preserve machine.features.diskQuotaSupport setting
// - preserve machine.features.apidCheckExtKeyUsage setting.
//
//nolint:gocognit,gocyclo,cyclop
func markVersionContract(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	clusterConfigVersionList, err := safe.StateListAll[*omni.ClusterConfigVersion](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list cluster config versions: %w", err)
	}

	clusterConfigVersionMap := make(map[string]string, clusterConfigVersionList.Len())
	for clusterConfigVersion := range clusterConfigVersionList.All() {
		clusterConfigVersionMap[clusterConfigVersion.Metadata().ID()] = clusterConfigVersion.TypedSpec().Value.Version
	}

	clusterMachineConfigList, err := safe.StateListAll[*omni.ClusterMachineConfig](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list cluster machine config patches: %w", err)
	}

	clusterMachineConfigStatusList, err := safe.StateListAll[*omni.ClusterMachineConfigStatus](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list cluster machine config status: %w", err)
	}

	configStatusMap := make(map[string]*omni.ClusterMachineConfigStatus, clusterMachineConfigStatusList.Len())
	for configStatus := range clusterMachineConfigStatusList.All() {
		configStatusMap[configStatus.Metadata().ID()] = configStatus
	}

	numNothingToDo, numConfigWasNotApplied, numDiskQuotaSupportEnabled, numApidCheckExtKeyUsageEnabled := 0, 0, 0, 0

	for clusterMachineConfig := range clusterMachineConfigList.All() {
		id := clusterMachineConfig.Metadata().ID()
		logger.Debug("checking cluster machine config patch", zap.String("id", id))

		if clusterMachineConfig.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		cluster, ok := clusterMachineConfig.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			logger.Warn("cluster machine config does not have a cluster label, skipping...", zap.String("id", clusterMachineConfig.Metadata().ID()))

			continue
		}

		clusterConfigVersion := clusterConfigVersionMap[cluster]
		if clusterConfigVersion == "" {
			logger.Warn("cluster does not have an initial version, skipping...", zap.String("id", clusterMachineConfig.Metadata().ID()), zap.String("cluster", cluster))

			continue
		}

		initialVersion, err := semver.ParseTolerant(clusterConfigVersion)
		if err != nil {
			logger.Warn("failed to parse initial version, skipping...", zap.String("id", clusterMachineConfig.Metadata().ID()), zap.String("cluster", cluster), zap.Error(err))

			continue
		}

		configStatus, ok := configStatusMap[id]
		if !ok {
			logger.Warn("cluster machine config status not found, skipping...", zap.String("id", clusterMachineConfig.Metadata().ID()))

			continue
		}

		if configStatus.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		if configStatus.TypedSpec().Value.ClusterMachineConfigVersion != clusterMachineConfig.Metadata().Version().String() {
			logger.Info("cluster machine config version does not match, buggy config was probably not applied, skipping...",
				zap.String("id", clusterMachineConfig.Metadata().ID()))

			numConfigWasNotApplied++

			continue
		}

		buffer, err := clusterMachineConfig.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return fmt.Errorf("failed to get uncompressed data: %w", err)
		}

		configDataStr := string(buffer.Data())

		buffer.Free()

		// see: https://github.com/siderolabs/omni/issues/1095
		preserveDiskQuotaSupport := initialVersion.Major == 1 && initialVersion.Minor <= 4 && strings.Contains(configDataStr, "diskQuotaSupport: true")
		preserveApidCheckExtKeyUsage := initialVersion.Major == 1 && initialVersion.Minor <= 2 && strings.Contains(configDataStr, "apidCheckExtKeyUsage: true")

		if !preserveDiskQuotaSupport && !preserveApidCheckExtKeyUsage {
			numNothingToDo++

			continue
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewClusterMachine(resources.DefaultNamespace, id).Metadata(), func(res *omni.ClusterMachine) error {
			if preserveApidCheckExtKeyUsage {
				res.Metadata().Annotations().Set(omni.PreserveApidCheckExtKeyUsage, "")
			}

			if preserveDiskQuotaSupport {
				res.Metadata().Annotations().Set(omni.PreserveDiskQuotaSupport, "")
			}

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(omnictrl.NewMachineSetStatusController().ControllerName))
		if err != nil && !state.IsPhaseConflictError(err) && !state.IsNotFoundError(err) {
			return err
		}
	}

	logger.Info("added annotations on the cluster machine to fix the version contract mismatch",
		zap.Int("num_total_processed", clusterMachineConfigList.Len()),
		zap.Int("num_config_was_not_applied", numConfigWasNotApplied),
		zap.Int("num_disk_quota_support_enabled", numDiskQuotaSupportEnabled),
		zap.Int("num_apid_check_ext_key_usage_enabled", numApidCheckExtKeyUsageEnabled),
		zap.Int("num_nothing_to_do", numNothingToDo),
	)

	return nil
}

func dropMachineClassStatusFinalizers(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	machineClasses, err := safe.StateListAll[*omni.MachineClass](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list cluster config versions: %w", err)
	}

	deprecatedFinalizer := "MachineClassStatusController"

	for machineClass := range machineClasses.All() {
		if !machineClass.Metadata().Finalizers().Has(deprecatedFinalizer) {
			continue
		}

		logger.Info("remove machine class status controller finalizer from the resource", zap.String("id", machineClass.Metadata().ID()))

		if err := st.RemoveFinalizer(ctx, machineClass.Metadata(), deprecatedFinalizer); err != nil {
			return err
		}
	}

	return nil
}

// createProviders creates infra.Provider resource for each existing infra.ProviderStatus.
// This migration is required to avoid breaking user's providers connection which were created before 0.50.
func createProviders(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	identities, err := st.List(ctx, auth.NewIdentity(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		return err
	}

	providers, err := st.List(ctx, infra.NewProviderStatus("").Metadata())
	if err != nil {
		return err
	}

	existingProviders := xslices.ToSet(
		xslices.Map(
			xslices.Filter(
				append(identities.Items, providers.Items...),
				func(res resource.Resource) bool {
					if res.Metadata().Phase() == resource.PhaseTearingDown {
						return false
					}

					if res.Metadata().Type() == auth.IdentityType && !strings.HasSuffix(res.Metadata().ID(), access.InfraProviderServiceAccountNameSuffix) {
						return false
					}

					return true
				},
			),
			func(res resource.Resource) string {
				if res.Metadata().Type() == auth.IdentityType {
					sa, _ := access.ParseServiceAccountFromFullID(res.Metadata().ID())

					return sa.BaseName
				}

				return res.Metadata().ID()
			},
		),
	)

	for id := range existingProviders {
		var provider *infra.Provider

		provider, err = safe.ReaderGetByID[*infra.Provider](ctx, st, id)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if provider != nil {
			continue
		}

		provider = infra.NewProvider(id)
		provider.Metadata().Labels().Set(omni.LabelInfraProviderID, id)

		logger.Info("register provider for the already existing provider status", zap.String("provider", id))

		if err = st.Create(ctx, provider); err != nil {
			return err
		}
	}

	return nil
}

//nolint:staticcheck
func migrateConnectionParamsToController(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	connectionParams, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, st, siderolink.ConfigID) //nolint:staticcheck
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if connectionParams.Metadata().Owner() == omnictrl.ConnectionParamsControllerName {
		return nil
	}

	_, err = safe.StateUpdateWithConflicts(ctx, st, connectionParams.Metadata(), func(res *siderolink.ConnectionParams) error { //nolint:staticcheck
		return res.Metadata().SetOwner(omnictrl.ConnectionParamsControllerName)
	}, state.WithExpectedPhaseAny())

	return err
}

// populateJoinTokenUsage starting from 1.0 every machine provision request creates JoinTokenUsage resource.
// Already existing machines won't call provision API, but should have resources created so we do it through migration.
// As there was no multiple join token support in Omni they can be simply populated with the default token.
func populateJoinTokenUsage(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	connectionParams, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, st, siderolink.ConfigID) //nolint:staticcheck
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	links, err := safe.ReaderListAll[*siderolink.Link](ctx, st)
	if err != nil {
		return err
	}

	for link := range links.All() {
		if err = safe.StateModify(ctx, st, siderolink.NewJoinTokenUsage(link.Metadata().ID()),
			func(res *siderolink.JoinTokenUsage) error {
				res.TypedSpec().Value.TokenId = connectionParams.TypedSpec().Value.JoinToken

				return nil
			},
		); err != nil {
			return err
		}
	}

	return err
}

func populateNodeUniqueTokens(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	links, err := safe.ReaderListAll[*siderolink.Link](ctx, st)
	if err != nil {
		return err
	}

	for link := range links.All() {
		if link.TypedSpec().Value.NodeUniqueToken == "" || link.Metadata().Phase() == resource.PhaseTearingDown { //nolint:staticcheck
			continue
		}

		if err = safe.StateModify(ctx, st, siderolink.NewNodeUniqueToken(link.Metadata().ID()),
			func(res *siderolink.NodeUniqueToken) error {
				res.TypedSpec().Value.Token = link.TypedSpec().Value.NodeUniqueToken //nolint:staticcheck

				return nil
			},
		); err != nil {
			return err
		}

		if err = safe.StateModify(ctx, st, link,
			func(res *siderolink.Link) error {
				res.TypedSpec().Value.NodeUniqueToken = "" //nolint:staticcheck

				return nil
			},
		); err != nil {
			return err
		}
	}

	return err
}

func moveClusterTaintFromResourceToLabel(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	clusterTaints, err := safe.ReaderListAll[*omni.ClusterTaint](ctx, st)
	if err != nil {
		return err
	}

	for taint := range clusterTaints.All() {
		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewClusterStatus(resources.DefaultNamespace, taint.Metadata().ID()).Metadata(), func(res *omni.ClusterStatus) error {
			res.Metadata().Labels().Set(omni.LabelClusterTaintedByBreakGlass, "")

			return nil
		}, state.WithExpectedPhaseAny(), state.WithUpdateOwner(omnictrl.ClusterStatusControllerName))
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}
		// cluster status does not exist, taint is dangling, just remove it

		if err = st.TeardownAndDestroy(ctx, taint.Metadata()); err != nil {
			return err
		}
	}

	return err
}

func dropExtraInputFinalizers(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	logger.Info("dropping extra finalizers from MachineSetStatus resources")

	if err := dropFinalizers[*omni.MachineSetStatus](ctx, st, "ClusterDestroyStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineSetStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from ClusterMachineStatus resources")

	if err := dropFinalizers[*omni.ClusterMachineStatus](ctx, st, "ClusterDestroyStatusController", "MachineSetDestroyStatusController", "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from ClusterMachineStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from Link resources")

	if err := dropFinalizers[*siderolink.Link](ctx, st, "BMCConfigController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from Link resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineSetNode resources")

	if err := dropFinalizers[*omni.MachineSetNode](ctx, st, "ClusterMachineStatusController", "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineSetNode resources: %w", err)
	}

	logger.Info("dropping extra finalizers from ClusterMachine resources")

	if err := dropFinalizers[*omni.ClusterMachine](ctx, st, "MachineStatusSnapshotController", "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from ClusterMachine resources: %w", err)
	}

	logger.Info("dropping extra finalizers from InfraMachineConfig resources")

	if err := dropFinalizers[*omni.InfraMachineConfig](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from InfraMachineConfig resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineExtensions resources")

	if err := dropFinalizers[*omni.MachineExtensions](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineExtensions resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineStatus resources")

	if err := dropFinalizers[*omni.MachineStatus](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineStatus resources: %w", err)
	}

	logger.Info("dropping extra finalizers from NodeUniqueToken resources")

	if err := dropFinalizers[*siderolink.NodeUniqueToken](ctx, st, "InfraMachineController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from NodeUniqueToken resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineStatusSnapshot resources")

	if err := dropFinalizers[*omni.MachineStatusSnapshot](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineStatusSnapshot resources: %w", err)
	}

	logger.Info("dropping extra finalizers from MachineLabels resources")

	if err := dropFinalizers[*omni.MachineLabels](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from MachineLabels resources: %w", err)
	}

	logger.Info("dropping extra finalizers from infra.MachineStatus resources")

	if err := dropFinalizers[*infra.MachineStatus](ctx, st, "MachineStatusController"); err != nil {
		return fmt.Errorf("failed to remove finalizers from infra.MachineStatus resources: %w", err)
	}

	return nil
}

func moveInfraProviderAnnotationsToLabels(ctx context.Context, st state.State, _ *zap.Logger, _ migrationContext) error {
	for _, resType := range []resource.Type{siderolink.LinkType, omni.MachineType} {
		kind := resource.NewMetadata(resources.DefaultNamespace, resType, "", resource.VersionUndefined)

		list, err := st.List(ctx, kind)
		if err != nil {
			return err
		}

		for _, item := range list.Items {
			id, ok := item.Metadata().Annotations().Get(omni.LabelInfraProviderID)
			if !ok {
				continue
			}

			if _, err = st.UpdateWithConflicts(ctx, item.Metadata(), func(res resource.Resource) error {
				res.Metadata().Labels().Set(omni.LabelInfraProviderID, id)
				res.Metadata().Annotations().Delete(omni.LabelInfraProviderID)

				return nil
			}, state.WithUpdateOwner(item.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func dropSchematicConfigFinalizerFromClusterMachines(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, s)
	if err != nil {
		return err
	}

	for cm := range list.All() {
		if cm.Metadata().Finalizers().Has(omnictrl.SchematicConfigurationControllerName) {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, cm.Metadata(), func(res *omni.ClusterMachine) error {
				res.Metadata().Finalizers().Remove(omnictrl.SchematicConfigurationControllerName)

				return nil
			}, state.WithUpdateOwner(cm.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func dropTalosUpgradeStatusFinalizersFromSchematicConfigs(ctx context.Context, s state.State, _ *zap.Logger, _ migrationContext) error {
	list, err := safe.ReaderListAll[*omni.SchematicConfiguration](ctx, s)
	if err != nil {
		return err
	}

	for sc := range list.All() {
		if sc.Metadata().Finalizers().Has(omnictrl.TalosUpgradeStatusControllerName) {
			if _, err = safe.StateUpdateWithConflicts(ctx, s, sc.Metadata(), func(res *omni.SchematicConfiguration) error {
				res.Metadata().Finalizers().Remove(omnictrl.TalosUpgradeStatusControllerName)

				return nil
			}, state.WithUpdateOwner(sc.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
				return err
			}
		}
	}

	return nil
}

func makeMachineSetNodesOwnerEmpty(ctx context.Context, st state.State, logger *zap.Logger, _ migrationContext) error {
	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, st)
	if err != nil {
		return err
	}

	for machineSetNode := range machineSetNodes.All() {
		if machineSetNode.Metadata().Owner() != omnictrl.NewMachineSetNodeController().ControllerName {
			continue
		}

		machineSet, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			continue
		}

		updated := omni.NewMachineSetNode(resources.DefaultNamespace, machineSetNode.Metadata().ID(),
			omni.NewMachineSet(resources.DefaultNamespace, machineSet),
		)

		for _, fin := range *machineSetNode.Metadata().Finalizers() {
			updated.Metadata().Finalizers().Add(fin)
		}

		updated.TypedSpec().Value = machineSetNode.TypedSpec().Value

		updated.Metadata().SetPhase(machineSetNode.Metadata().Phase())
		updated.Metadata().SetVersion(machineSetNode.Metadata().Version())

		updated.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for key, value := range machineSetNode.Metadata().Labels().Raw() {
				temp.Set(key, value)
			}
		})

		updated.Metadata().Annotations().Do(func(temp kvutils.TempKV) {
			for key, value := range machineSetNode.Metadata().Annotations().Raw() {
				temp.Set(key, value)
			}
		})

		updated.Metadata().Labels().Set(omni.LabelManagedByMachineSetNodeController, "")

		if err = updated.Metadata().SetOwner(""); err != nil {
			return err
		}

		if err = st.Update(ctx, updated, state.WithUpdateOwner(machineSetNode.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
			return err
		}
	}

	return nil
}
