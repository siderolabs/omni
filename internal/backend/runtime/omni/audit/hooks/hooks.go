// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package hooks define a set of hooks that can be used to audit events in the system.
package hooks

import (
	"context"
	"errors"
	"maps"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// Init initializes the audit hooks.
func Init(a *audit.Log) {
	audit.ShouldLogCreate(a, publicKeyCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, publicKeyUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, publicKeyUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.PublicKeyType, publicKeyDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, userCreate, audit.WithInternalAgent())
	audit.ShouldLogCreate(a, identityCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, userUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, identityUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, userUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, identityUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.UserType, userDestroy, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.IdentityType, identityDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, machineCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, machineUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, machineUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineType, machineDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, machineLabelsCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, machineLabelsUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, machineLabelsUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineLabelsType, machineLabelsDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, accessPolicyCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, accessPolicyUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, accessPolicyUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, auth.AccessPolicyType, accessPolicyDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, clusterCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, clusterUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, clusterUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.ClusterType, clusterDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, machineSetCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, machineSetUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, machineSetUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineSetType, machineSetDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, machineSetNodeCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, machineSetNodeUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, machineSetNodeUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.MachineSetNodeType, machineSetNodeDestroy, audit.WithInternalAgent())

	audit.ShouldLogCreate(a, configPatchCreate, audit.WithInternalAgent())
	audit.ShouldLogUpdate(a, configPatchUpdate, audit.WithInternalAgent())
	audit.ShouldLogUpdateWithConflicts(a, configPatchUpdate, audit.WithInternalAgent())
	audit.ShouldLogDestroy(a, omni.ConfigPatchType, configPatchDestroy, audit.WithInternalAgent())
}

func publicKeyCreate(_ context.Context, data *audit.Data, res *auth.PublicKey, _ ...state.CreateOption) error {
	return handlePublicKey(data, res)
}

func publicKeyUpdate(_ context.Context, data *audit.Data, _, newRes *auth.PublicKey, _ ...state.UpdateOption) error {
	return handlePublicKey(data, newRes)
}

func publicKeyDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	data.Session.Fingerprint = ptr.ID()

	return nil
}

func handlePublicKey(data *audit.Data, res *auth.PublicKey) error {
	userID, ok := res.Metadata().Labels().Get(auth.LabelPublicKeyUserID)
	if !ok {
		return errors.New("missing user ID on public key creation")
	}

	r, err := role.Parse(res.TypedSpec().Value.GetRole())
	if err != nil {
		return err
	}

	data.Session.Fingerprint = res.Metadata().ID()
	data.Session.UserID = userID
	data.Session.Email = res.TypedSpec().Value.GetIdentity().GetEmail()
	data.Session.Role = r
	data.Session.PublicKeyExpiration = res.TypedSpec().Value.GetExpiration().Seconds

	return nil
}

func userCreate(_ context.Context, data *audit.Data, res *auth.User, _ ...state.CreateOption) error {
	return handleUser(data, res)
}

func userUpdate(_ context.Context, data *audit.Data, _, newRes *auth.User, _ ...state.UpdateOption) error {
	return handleUser(data, newRes)
}

func handleUser(data *audit.Data, res *auth.User) error {
	initPtrField(&data.NewUser)

	data.NewUser.Role = role.Role(res.TypedSpec().Value.Role)
	data.NewUser.UserID = res.Metadata().ID()

	return nil
}

func userDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.NewUser)

	data.NewUser.UserID = ptr.ID()

	return nil
}

func identityCreate(_ context.Context, data *audit.Data, res *auth.Identity, _ ...state.CreateOption) error {
	return handleIdentity(data, res)
}

func identityUpdate(_ context.Context, data *audit.Data, _, newRes *auth.Identity, _ ...state.UpdateOption) error {
	return handleIdentity(data, newRes)
}

func handleIdentity(data *audit.Data, res *auth.Identity) error {
	initPtrField(&data.NewUser)

	data.NewUser.Email = res.Metadata().ID()
	data.NewUser.UserID = res.TypedSpec().Value.GetUserId()
	data.NewUser.IsServiceAccount = isServiceAccount(res.Metadata())

	return nil
}

func identityDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
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

func machineCreate(_ context.Context, data *audit.Data, res *omni.Machine, _ ...state.CreateOption) error {
	return handleMachine(data, res)
}

func machineUpdate(_ context.Context, data *audit.Data, _, newRes *omni.Machine, _ ...state.UpdateOption) error {
	if newRes.Metadata().Phase() != resource.PhaseTearingDown {
		return audit.ErrNoLog
	}

	return handleMachine(data, newRes)
}

func handleMachine(data *audit.Data, res *omni.Machine) error {
	initPtrField(&data.Machine)

	data.Machine.ID = res.Metadata().ID()
	data.Machine.IsConnected = res.TypedSpec().Value.GetConnected()
	data.Machine.ManagementAddress = res.TypedSpec().Value.GetManagementAddress()
	data.Machine.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func machineDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.Machine)

	data.Machine.ID = ptr.ID()

	md, ok := ptr.(*resource.Metadata)
	if !ok {
		return nil
	}

	data.Machine.Labels = maps.Clone(md.Labels().Raw())

	return nil
}

func machineLabelsCreate(_ context.Context, data *audit.Data, res *omni.MachineLabels, _ ...state.CreateOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = res.Metadata().ID()
	data.MachineLabels.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func machineLabelsUpdate(_ context.Context, data *audit.Data, _, newRes *omni.MachineLabels, _ ...state.UpdateOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = newRes.Metadata().ID()
	data.MachineLabels.Labels = maps.Clone(newRes.Metadata().Labels().Raw())

	return nil
}

func machineLabelsDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineLabels)

	data.MachineLabels.ID = ptr.ID()

	return nil
}

func accessPolicyCreate(_ context.Context, data *audit.Data, res *auth.AccessPolicy, _ ...state.CreateOption) error {
	return handleAccessPolicy(data, res)
}

func accessPolicyUpdate(_ context.Context, data *audit.Data, _, newRes *auth.AccessPolicy, _ ...state.UpdateOption) error {
	return handleAccessPolicy(data, newRes)
}

func handleAccessPolicy(data *audit.Data, res *auth.AccessPolicy) error {
	initPtrField(&data.AccessPolicy)

	data.AccessPolicy.ID = res.Metadata().ID()
	data.AccessPolicy.ClusterGroups = res.TypedSpec().Value.GetClusterGroups()
	data.AccessPolicy.UserGroups = res.TypedSpec().Value.GetUserGroups()
	data.AccessPolicy.Rules = res.TypedSpec().Value.GetRules()
	data.AccessPolicy.Tests = res.TypedSpec().Value.GetTests()

	return nil
}

func accessPolicyDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.AccessPolicy)

	data.AccessPolicy.ID = ptr.ID()

	return nil
}

func clusterCreate(_ context.Context, data *audit.Data, res *omni.Cluster, _ ...state.CreateOption) error {
	return handleCluster(data, res)
}

func clusterUpdate(_ context.Context, data *audit.Data, _, newRes *omni.Cluster, _ ...state.UpdateOption) error {
	return handleCluster(data, newRes)
}

func handleCluster(data *audit.Data, res *omni.Cluster) error {
	initPtrField(&data.Cluster)

	data.Cluster.ID = res.Metadata().ID()
	data.Cluster.BackupConfiguration = res.TypedSpec().Value.GetBackupConfiguration()
	data.Cluster.Features = res.TypedSpec().Value.GetFeatures()
	data.Cluster.KubernetesVersion = res.TypedSpec().Value.GetKubernetesVersion()
	data.Cluster.TalosVersion = res.TypedSpec().Value.GetTalosVersion()
	data.Cluster.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func clusterDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.Cluster)

	data.Cluster.ID = ptr.ID()

	return nil
}

func machineSetCreate(_ context.Context, data *audit.Data, res *omni.MachineSet, _ ...state.CreateOption) error {
	return handleMachineSet(data, res, true)
}

func machineSetUpdate(_ context.Context, data *audit.Data, _, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
	return handleMachineSet(data, newRes, newRes.Metadata().Owner() == "")
}

func handleMachineSet(data *audit.Data, res *omni.MachineSet, emptyOwner bool) error {
	if !emptyOwner {
		return audit.ErrNoLog
	}

	initPtrField(&data.MachineSet)

	data.MachineSet.ID = res.Metadata().ID()
	data.MachineSet.UpdateStrategy = res.TypedSpec().Value.GetUpdateStrategy().String()
	data.MachineSet.MachineClass = res.TypedSpec().Value.GetMachineClass()
	data.MachineSet.BootstrapSpec = res.TypedSpec().Value.GetBootstrapSpec()
	data.MachineSet.DeleteStrategy = res.TypedSpec().Value.GetDeleteStrategy().String()
	data.MachineSet.UpdateStrategyConfig = res.TypedSpec().Value.GetUpdateStrategyConfig()
	data.MachineSet.DeleteStrategyConfig = res.TypedSpec().Value.GetDeleteStrategyConfig()
	data.MachineSet.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func machineSetDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineSet)

	data.MachineSet.ID = ptr.ID()

	return nil
}

func machineSetNodeCreate(_ context.Context, data *audit.Data, res *omni.MachineSetNode, _ ...state.CreateOption) error {
	return handleMachineSetNode(data, res, true)
}

func machineSetNodeUpdate(_ context.Context, data *audit.Data, _, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
	return handleMachineSetNode(data, newRes, newRes.Metadata().Owner() == "")
}

func handleMachineSetNode(data *audit.Data, res *omni.MachineSetNode, emptyOwner bool) error {
	if !emptyOwner {
		return audit.ErrNoLog
	}

	initPtrField(&data.MachineSetNode)

	data.MachineSetNode.ID = res.Metadata().ID()
	data.MachineSetNode.Labels = maps.Clone(res.Metadata().Labels().Raw())

	return nil
}

func machineSetNodeDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.MachineSetNode)

	data.MachineSetNode.ID = ptr.ID()

	return nil
}

func configPatchCreate(_ context.Context, data *audit.Data, res *omni.ConfigPatch, _ ...state.CreateOption) error {
	return handleConfigPatch(data, res)
}

func configPatchUpdate(_ context.Context, data *audit.Data, _, newRes *omni.ConfigPatch, _ ...state.UpdateOption) error {
	return handleConfigPatch(data, newRes)
}

func handleConfigPatch(data *audit.Data, res *omni.ConfigPatch) error {
	initPtrField(&data.ConfigPatch)

	data.ConfigPatch.ID = res.Metadata().ID()
	data.ConfigPatch.Labels = maps.Clone(res.Metadata().Labels().Raw())
	data.ConfigPatch.Data = res.TypedSpec().Value.GetData()

	return nil
}

func configPatchDestroy(_ context.Context, data *audit.Data, ptr resource.Pointer, _ ...state.DestroyOption) error {
	initPtrField(&data.ConfigPatch)

	data.ConfigPatch.ID = ptr.ID()

	return nil
}

func initPtrField[T any](v **T) {
	if *v == nil {
		*v = new(T)
	}
}
