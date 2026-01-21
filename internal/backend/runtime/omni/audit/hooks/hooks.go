// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package hooks define a set of hooks that can be used to audit events in the system.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// Init initializes the audit hooks.
func Init(a *audit.Log) {
	audit.ShouldLogCreate(a, auth.PublicKeyType, publicKeyCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, auth.PublicKeyType, publicKeyUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, auth.PublicKeyType, publicKeyUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.PublicKeyType, publicKeyDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, auth.UserType, userCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, auth.UserType, userUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, auth.UserType, userUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.UserType, userDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, auth.IdentityType, identityCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, auth.IdentityType, identityUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, auth.IdentityType, identityUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.IdentityType, identityDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.MachineType, machineCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.MachineType, machineUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.MachineType, machineUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineType, machineDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.MachineLabelsType, machineLabelsCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.MachineLabelsType, machineLabelsUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.MachineLabelsType, machineLabelsUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineLabelsType, machineLabelsDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, auth.AccessPolicyType, accessPolicyCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, auth.AccessPolicyType, accessPolicyUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, auth.AccessPolicyType, accessPolicyUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.AccessPolicyType, accessPolicyDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.ClusterType, clusterCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.ClusterType, clusterUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.ClusterType, clusterUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.ClusterType, clusterDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.MachineSetType, machineSetCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.MachineSetType, machineSetUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.MachineSetType, machineSetUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineSetType, machineSetDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.MachineSetNodeType, machineSetNodeCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.MachineSetNodeType, machineSetNodeUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.MachineSetNodeType, machineSetNodeUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineSetNodeType, machineSetNodeDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.ConfigPatchType, configPatchCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, omni.ConfigPatchType, configPatchUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, omni.ConfigPatchType, configPatchUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.ConfigPatchType, configPatchDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, omni.MachineConfigDiffType, machineConfigDiffCreate, audit.WithInternalAgent())

	emptyCreateFunc := func(_ context.Context, _ *auditlog.Data, _ resource.Resource, _ ...state.CreateOption) error {
		return nil
	}
	emptyUpdateFunc := func(_ context.Context, _ *auditlog.Data, _, _ resource.Resource, _ ...state.UpdateOption) error {
		return nil
	}
	emptyDestroyFunc := func(_ context.Context, _ *auditlog.Data, _ resource.Pointer, _ ...state.DestroyOption) error {
		return nil
	}

	customLoggedResourceTypes := xslices.ToSet(slices.Concat(a.CreateHooksResourceTypes(), a.UpdateHooksResourceTypes(), a.DestroyHooksResourceTypes()))

	userManagedResourceTypes := common.UserManagedResourceTypes
	for _, rt := range userManagedResourceTypes {
		if _, ok := customLoggedResourceTypes[rt]; ok {
			continue
		}

		audit.ShouldLogCreate(a, rt, emptyCreateFunc, audit.WithInternalAgent())
		audit.ShouldLogUpdate(a, rt, emptyUpdateFunc, audit.WithInternalAgent())
		audit.ShouldLogUpdateWithConflicts(a, rt, emptyUpdateFunc, audit.WithInternalAgent())
		audit.ShouldLogDestroy(a, rt, emptyDestroyFunc, audit.WithInternalAgent())
	}
}

func publicKeyCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handlePublicKey(data, res)
}

func publicKeyUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handlePublicKey(data, newRes)
}

func publicKeyDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	data.Session.Fingerprint = ptr.ID()

	return nil
}

func handlePublicKey(data *auditlog.Data, res resource.Resource) error {
	publicKey, ok := res.(*auth.PublicKey)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	userID, ok := res.Metadata().Labels().Get(auth.LabelPublicKeyUserID)
	if !ok {
		return errors.New("missing user ID on public key creation")
	}

	r, err := role.Parse(publicKey.TypedSpec().Value.GetRole())
	if err != nil {
		return err
	}

	data.Session.Fingerprint = res.Metadata().ID()
	data.Session.UserID = userID
	data.Session.Email = publicKey.TypedSpec().Value.GetIdentity().GetEmail()
	data.Session.Role = r
	data.Session.PublicKeyExpiration = publicKey.TypedSpec().Value.GetExpiration().Seconds

	return nil
}

func userCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleUser(data, res)
}

func userUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleUser(data, newRes)
}

func handleUser(data *auditlog.Data, res resource.Resource) error {
	user, ok := res.(*auth.User)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.NewUser)

	data.NewUser.Role = role.Role(user.TypedSpec().Value.Role)
	data.NewUser.UserID = user.Metadata().ID()

	return nil
}

func userDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.NewUser)

	data.NewUser.UserID = ptr.ID()

	return nil
}

func identityCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleIdentity(data, res)
}

func identityUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleIdentity(data, newRes)
}

func handleIdentity(data *auditlog.Data, res resource.Resource) error {
	identity, ok := res.(*auth.Identity)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.NewUser)

	data.NewUser.Email = identity.Metadata().ID()
	data.NewUser.UserID = identity.TypedSpec().Value.GetUserId()
	data.NewUser.IsServiceAccount = isServiceAccount(identity.Metadata())

	return nil
}

func identityDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.NewUser)

	data.NewUser.Email = ptr.ID()

	md, ok := ptr.(*resource.Metadata)
	if !ok {
		return nil
	}

	data.NewUser.IsServiceAccount = isServiceAccount(md)

	return nil
}

func isServiceAccount(md *resource.Metadata) bool {
	_, isServiceAccount := md.Labels().Get(auth.LabelIdentityTypeServiceAccount)

	return isServiceAccount
}

func machineCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleMachine(data, res)
}

func machineUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	if newRes.Metadata().Phase() != resource.PhaseTearingDown {
		return audit.ErrNoLog
	}

	return handleMachine(data, newRes)
}

func handleMachine(data *auditlog.Data, res resource.Resource) error {
	machine, ok := res.(*omni.Machine)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.Machine)

	data.Machine.ID = machine.Metadata().ID()
	data.Machine.IsConnected = machine.TypedSpec().Value.GetConnected()
	data.Machine.ManagementAddress = machine.TypedSpec().Value.GetManagementAddress()
	data.Machine.Labels = maps.Clone(machine.Metadata().Labels().Raw())

	return nil
}

func machineDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.Machine)

	data.Machine.ID = ptr.ID()

	md, ok := ptr.(*resource.Metadata)
	if !ok {
		return nil
	}

	data.Machine.Labels = maps.Clone(md.Labels().Raw())

	return nil
}

func machineLabelsCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = res.Metadata().ID()
	data.MachineLabels.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func machineLabelsUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = newRes.Metadata().ID()
	data.MachineLabels.Labels = maps.Clone(newRes.Metadata().Labels().Raw())

	return nil
}

func machineLabelsDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = ptr.ID()

	return nil
}

func accessPolicyCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleAccessPolicy(data, res)
}

func accessPolicyUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleAccessPolicy(data, newRes)
}

func handleAccessPolicy(data *auditlog.Data, res resource.Resource) error {
	accessPolicy, ok := res.(*auth.AccessPolicy)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.AccessPolicy)

	data.AccessPolicy.ID = res.Metadata().ID()
	data.AccessPolicy.ClusterGroups = accessPolicy.TypedSpec().Value.GetClusterGroups()
	data.AccessPolicy.UserGroups = accessPolicy.TypedSpec().Value.GetUserGroups()
	data.AccessPolicy.Rules = accessPolicy.TypedSpec().Value.GetRules()
	data.AccessPolicy.Tests = accessPolicy.TypedSpec().Value.GetTests()

	return nil
}

func accessPolicyDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.AccessPolicy)

	data.AccessPolicy.ID = ptr.ID()

	return nil
}

func clusterCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleCluster(data, res)
}

func clusterUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleCluster(data, newRes)
}

func handleCluster(data *auditlog.Data, res resource.Resource) error {
	cluster, ok := res.(*omni.Cluster)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.Cluster)

	data.Cluster.ID = res.Metadata().ID()
	data.Cluster.BackupConfiguration = cluster.TypedSpec().Value.GetBackupConfiguration()
	data.Cluster.Features = cluster.TypedSpec().Value.GetFeatures()
	data.Cluster.KubernetesVersion = cluster.TypedSpec().Value.GetKubernetesVersion()
	data.Cluster.TalosVersion = cluster.TypedSpec().Value.GetTalosVersion()
	data.Cluster.Labels = maps.Clone(cluster.Metadata().Labels().Raw())

	return nil
}

func clusterDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.Cluster)

	data.Cluster.ID = ptr.ID()

	return nil
}

func machineSetCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleMachineSet(data, res, true)
}

func machineSetUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleMachineSet(data, newRes, newRes.Metadata().Owner() == "")
}

func handleMachineSet(data *auditlog.Data, res resource.Resource, emptyOwner bool) error {
	machineSet, ok := res.(*omni.MachineSet)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	if !emptyOwner {
		return audit.ErrNoLog
	}

	initPtrField(&data.MachineSet)

	data.MachineSet.ID = res.Metadata().ID()
	data.MachineSet.UpdateStrategy = machineSet.TypedSpec().Value.GetUpdateStrategy().String()
	data.MachineSet.MachineAllocation = omni.GetMachineAllocation(machineSet)
	data.MachineSet.BootstrapSpec = machineSet.TypedSpec().Value.GetBootstrapSpec()
	data.MachineSet.DeleteStrategy = machineSet.TypedSpec().Value.GetDeleteStrategy().String()
	data.MachineSet.UpdateStrategyConfig = machineSet.TypedSpec().Value.GetUpdateStrategyConfig()
	data.MachineSet.DeleteStrategyConfig = machineSet.TypedSpec().Value.GetDeleteStrategyConfig()
	data.MachineSet.Labels = maps.Clone(machineSet.Metadata().Labels().Raw())
	data.MachineSet.ClusterID, _ = machineSet.Metadata().Labels().Get(omni.LabelCluster)

	return nil
}

func machineSetDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineSet)

	data.MachineSet.ID = ptr.ID()

	// We cannot extract ClusterID without reading the resource, so we skip it here.

	return nil
}

func machineSetNodeCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleMachineSetNode(data, res, true)
}

func machineSetNodeUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleMachineSetNode(data, newRes, newRes.Metadata().Owner() == "")
}

func handleMachineSetNode(data *auditlog.Data, res resource.Resource, emptyOwner bool) error {
	if !emptyOwner {
		return audit.ErrNoLog
	}

	initPtrField(&data.MachineSetNode)

	data.MachineSetNode.ID = res.Metadata().ID()
	data.MachineSetNode.Labels = maps.Clone(res.Metadata().Labels().Raw())
	data.MachineSetNode.ClusterID, _ = res.Metadata().Labels().Get(omni.LabelCluster)

	return nil
}

func machineSetNodeDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineSetNode)

	data.MachineSetNode.ID = ptr.ID()

	// We cannot extract ClusterID without reading the resource, so we skip it here.

	return nil
}

func configPatchCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	return handleConfigPatch(data, res)
}

func configPatchUpdate(_ context.Context, data *auditlog.Data, _, newRes resource.Resource, _ ...state.UpdateOption) error {
	return handleConfigPatch(data, newRes)
}

func handleConfigPatch(data *auditlog.Data, res resource.Resource) error {
	configPatch, ok := res.(*omni.ConfigPatch)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.ConfigPatch)

	data.ConfigPatch.ID = res.Metadata().ID()
	data.ConfigPatch.Labels = maps.Clone(res.Metadata().Labels().Raw())
	data.ConfigPatch.ClusterID, _ = res.Metadata().Labels().Get(omni.LabelCluster)

	buffer, err := configPatch.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return err
	}

	defer buffer.Free()

	data.ConfigPatch.Data = string(buffer.Data())

	return nil
}

func configPatchDestroy(_ context.Context, data *auditlog.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.ConfigPatch)

	data.ConfigPatch.ID = ptr.ID()

	// We cannot extract ClusterID without reading the resource, so we skip it here.

	return nil
}

func machineConfigDiffCreate(_ context.Context, data *auditlog.Data, res resource.Resource, _ ...state.CreateOption) error {
	machineConfigDiff, ok := res.(*omni.MachineConfigDiff)
	if !ok {
		return fmt.Errorf("unexpected type: %q", res.Metadata().Type())
	}

	initPtrField(&data.MachineConfigDiff)

	data.MachineConfigDiff.ID = machineConfigDiff.Metadata().ID()
	data.MachineConfigDiff.Diff = machineConfigDiff.TypedSpec().Value.Diff
	data.MachineConfigDiff.ClusterID, _ = machineConfigDiff.Metadata().Labels().Get(omni.LabelCluster)

	return nil
}

func initPtrField[T any](v **T) {
	if *v == nil {
		*v = new(T)
	}
}
