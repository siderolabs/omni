// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/pair"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/auth/scope"
	"github.com/siderolabs/omni/internal/pkg/image"
)

const (
	deprecatedControlPlaneRole = "role-controlplane"
	deprecatedWorkerRole       = "role-worker"
	deprecatedCluster          = "cluster"
)

// populate Kubernetes versions and Talos versions ClusterMachineTemplates in the Cluster resources.
func clusterInfo(ctx context.Context, s state.State, _ *zap.Logger) error {
	list, err := safe.StateListAll[*omni.Cluster](ctx, s)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value().TypedSpec()

		if item.Value.InstallImage == "" || item.Value.KubernetesVersion == "" { //nolint:staticcheck
			_, err = safe.StateUpdateWithConflicts(ctx, s, iter.Value().Metadata(), func(c *omni.Cluster) error {
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

func deprecateClusterMachineTemplates(ctx context.Context, s state.State, _ *zap.Logger) error {
	list, err := safe.StateListAll[*omni.ClusterMachineTemplate](ctx, s)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value().TypedSpec()

		// generate user patch if it is defined
		if item.Value.Patch != "" {
			patch := omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("500-%s", uuid.New()),
				pair.MakePair("machine-uuid", iter.Value().Metadata().ID()),
			)

			helpers.CopyLabels(iter.Value(), patch, deprecatedCluster)

			if err = createOrUpdate(ctx, s, patch, func(p *omni.ConfigPatch) error {
				p.TypedSpec().Value.Data = item.Value.Patch

				return nil
			}, ""); err != nil {
				return err
			}
		}

		// generate install disk patch
		patch := omni.NewConfigPatch(
			resources.DefaultNamespace,
			fmt.Sprintf("000-%s", uuid.New()),
			pair.MakePair("machine-uuid", iter.Value().Metadata().ID()),
		)

		helpers.CopyLabels(iter.Value(), patch, deprecatedCluster)

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

			p.TypedSpec().Value.Data = string(data)
			p.Metadata().Labels().Set("machine-uuid", iter.Value().Metadata().ID())

			return nil
		}, ""); err != nil {
			return err
		}
	}

	return nil
}

func clusterMachinesToMachineSets(ctx context.Context, s state.State, logger *zap.Logger) error {
	list, err := safe.StateListAll[*omni.ClusterMachine](ctx, s)
	if err != nil {
		return err
	}

	machineSets := map[string]*omni.MachineSet{}

	// first gather all machines from the database and create backup data
	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value()

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

func changePublicKeyOwner(ctx context.Context, s state.State, logger *zap.Logger) error {
	logger = logger.With(zap.String("migration", "changePublicKeyOwner"))

	list, err := safe.StateListAll[*auth.PublicKey](ctx, s)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value()

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

func addDefaultScopesToUsers(ctx context.Context, s state.State, logger *zap.Logger) error {
	logger = logger.With(zap.String("migration", "addDefaultScopesToUsers"))

	list, err := safe.StateListAll[*auth.User](ctx, s)
	if err != nil {
		return err
	}

	scopes := scope.NewScopes(scope.UserDefaultScopes...).Strings()

	for iter := list.Iterator(); iter.Next(); {
		user := iter.Value()

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

func setRollingStrategyOnControlPlaneMachineSets(ctx context.Context, s state.State, logger *zap.Logger) error {
	logger = logger.With(zap.String("migration", "setRollingStrategyOnControlPlaneMachineSets"))

	list, err := safe.StateListAll[*omni.MachineSet](
		ctx,
		s,
		state.WithLabelQuery(resource.LabelExists(deprecatedControlPlaneRole)),
	)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		machineSet := iter.Value()

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

func updateConfigPatchLabels(ctx context.Context, s state.State, _ *zap.Logger) error {
	clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](
		ctx,
		s,
	)
	if err != nil {
		return err
	}

	clusterMachineToCluster := map[resource.ID]resource.ID{}

	for iter := clusterMachineList.Iterator(); iter.Next(); {
		clusterMachine := iter.Value()

		cluster, _ := clusterMachine.Metadata().Labels().Get(deprecatedCluster)
		clusterMachineToCluster[clusterMachine.Metadata().ID()] = cluster
	}

	machineSetList, err := safe.StateListAll[*omni.MachineSet](ctx, s)
	if err != nil {
		return err
	}

	machineSetToCluster := map[resource.ID]resource.ID{}

	for iter := machineSetList.Iterator(); iter.Next(); {
		machineSet := iter.Value()

		cluster, _ := machineSet.Metadata().Labels().Get(deprecatedCluster)
		machineSetToCluster[machineSet.Metadata().ID()] = cluster
	}

	configPatchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, s)
	if err != nil {
		return err
	}

	for iter := configPatchList.Iterator(); iter.Next(); {
		_, err = safe.StateUpdateWithConflicts(ctx, s, iter.Value().Metadata(), func(patch *omni.ConfigPatch) error {
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
func updateMachineFinalizers(ctx context.Context, s state.State, logger *zap.Logger) error {
	logger = logger.With(zap.String("migration", "updateMachineFinalizers"))

	machineList, err := safe.StateListAll[*omni.Machine](ctx, s)
	if err != nil {
		return err
	}

	for iter := machineList.Iterator(); iter.Next(); {
		machine := iter.Value()

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

func labelConfigPatches(ctx context.Context, s state.State, _ *zap.Logger) error {
	patchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, s)
	if err != nil {
		return err
	}

	for iter := patchList.Iterator(); iter.Next(); {
		patch := iter.Value()

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

func updateMachineStatusClusterRelations(ctx context.Context, s state.State, _ *zap.Logger) error {
	msList, err := safe.StateListAll[*omni.MachineStatus](ctx, s)
	if err != nil {
		return err
	}

	for iter := msList.Iterator(); iter.Next(); {
		ms := iter.Value()

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
func addServiceAccountScopesToUsers(ctx context.Context, s state.State, _ *zap.Logger) error {
	userList, err := safe.StateListAll[*auth.User](ctx, s)
	if err != nil {
		return err
	}

	for iter := userList.Iterator(); iter.Next(); {
		user := iter.Value()

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

	for iter := publicKeyList.Iterator(); iter.Next(); {
		key := iter.Value()

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

func clusterInstallImageToTalosVersion(ctx context.Context, s state.State, _ *zap.Logger) error {
	clusterList, err := safe.StateListAll[*omni.Cluster](ctx, s)
	if err != nil {
		return err
	}

	for iter := clusterList.Iterator(); iter.Next(); {
		cluster := iter.Value()

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

func migrateLabels(ctx context.Context, s state.State, _ *zap.Logger) error {
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

func dropOldLabels(ctx context.Context, s state.State, _ *zap.Logger) error {
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

func convertScopesToRoles(ctx context.Context, st state.State, _ *zap.Logger) error {
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

	for iter := userList.Iterator(); iter.Next(); {
		item := iter.Value()

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

	for iter := pubKeyList.Iterator(); iter.Next(); {
		item := iter.Value()

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

func lowercaseAllIdentities(ctx context.Context, st state.State, _ *zap.Logger) error {
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

func removeConfigPatchesFromClusterMachines(ctx context.Context, st state.State, _ *zap.Logger) error {
	items, err := safe.ReaderListAll[*omni.ClusterMachine](
		ctx,
		st,
	)
	if err != nil {
		return err
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
					patchesRaw = append(patchesRaw, p.TypedSpec().Value.Data)
				}

				res.TypedSpec().Value.Patches = patchesRaw

				return nil
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
func machineInstallDiskPatches(ctx context.Context, st state.State, _ *zap.Logger) error {
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
func siderolinkCounters(ctx context.Context, s state.State, logger *zap.Logger) error {
	logger = logger.With(zap.String("migration", "siderolinkCounters"))

	list, err := safe.StateListAll[*siderolink.DeprecatedLinkCounter](ctx, s)
	if err != nil {
		return err
	}

	migrated := 0

	for iter := list.Iterator(); iter.Next(); {
		linkCounter := iter.Value()

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
func fixClusterTalosVersionOwnership(ctx context.Context, s state.State, _ *zap.Logger) error {
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
func updateClusterMachineConfigPatchesLabels(ctx context.Context, s state.State, _ *zap.Logger) error {
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
func clearEmptyConfigPatches(ctx context.Context, s state.State, _ *zap.Logger) error {
	list, err := safe.StateListAll[*omni.ClusterMachineConfigPatches](ctx, s)

	return list.ForEachErr(func(res *omni.ClusterMachineConfigPatches) error {
		if len(res.TypedSpec().Value.Patches) == 0 {
			return nil
		}

		var filteredPatches []string

		for _, patch := range res.TypedSpec().Value.Patches {
			if strings.TrimSpace(patch) != "" {
				filteredPatches = append(filteredPatches, patch)
			}
		}

		if len(filteredPatches) == len(res.TypedSpec().Value.Patches) {
			return nil
		}

		_, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), func(r *omni.ClusterMachineConfigPatches) error {
			r.TypedSpec().Value.Patches = filteredPatches

			return nil
		}, state.WithUpdateOwner(res.Metadata().Owner()), state.WithExpectedPhaseAny())

		return err
	})
}

// removes schematic configurations which are not associated with the machines.
func cleanupDanglingSchematicConfigurations(ctx context.Context, s state.State, _ *zap.Logger) error {
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

func cleanupExtensionsConfigurationStatuses(ctx context.Context, s state.State, _ *zap.Logger) error {
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

func dropSchematicConfigurationsControllerFinalizer(ctx context.Context, s state.State, _ *zap.Logger) error {
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
func generateAllMaintenanceConfigs(context.Context, state.State, *zap.Logger) error {
	// deprecated
	return nil
}

// setMachineStatusSnapshotOwner reconciles maintenance configs for all machines and update the inputs to avoid triggering config updates for each machine.
func setMachineStatusSnapshotOwner(ctx context.Context, st state.State, logger *zap.Logger) error {
	list, err := safe.StateListAll[*omni.MachineStatusSnapshot](ctx, st)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value()

		if item.Metadata().Owner() != "" {
			continue
		}

		logger.Info("updating machine status snapshot with empty owner", zap.String("id", item.Metadata().String()))

		_, err = safe.StateUpdateWithConflicts(ctx, st, item.Metadata(), func(res *omni.MachineStatusSnapshot) error {
			return res.Metadata().SetOwner(omnictrl.NewMachineStatusSnapshotController(nil).Name())
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
func migrateInstallImageConfigIntoGenOptions(ctx context.Context, st state.State, _ *zap.Logger) error {
	list, err := safe.StateListAll[*omni.MachineConfigGenOptions](ctx, st)
	if err != nil {
		return err
	}

	for iter := list.Iterator(); iter.Next(); {
		genOptions := iter.Value()

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
func dropGeneratedMaintenanceConfigs(ctx context.Context, st state.State, _ *zap.Logger) error {
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
