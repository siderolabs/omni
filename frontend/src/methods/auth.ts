// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import {
  CurrentUserID,
  CurrentUserType,
  VirtualNamespace,
  PermissionsType,
  PermissionsID,
  ClusterPermissionsType
} from "@/api/resources";

import { ResourceService, Resource } from "@/api/grpc";
import { Runtime } from "@/api/common/omni.pb";
import { CurrentUserSpec, PermissionsSpec } from "@/api/omni/specs/virtual.pb";
import { computed, ComputedRef, onBeforeMount, ref, Ref, watch } from "vue";
import { withRuntime } from "@/api/options";

export const currentUser: Ref<Resource<CurrentUserSpec> | undefined> = ref();
export const permissions: Ref<Resource<PermissionsSpec> | undefined> = ref();

export const setupClusterPermissions = (cluster: {value: string}) => {
  const result = {
    canUpdateKubernetes: ref(false),
    canUpdateTalos: ref(false),
    canDownloadTalosconfig: ref(false),
    canDownloadKubeconfig: ref(false),
    canDownloadSupportBundle: ref(false),
    canAddClusterMachines: ref(false),
    canRemoveClusterMachines: ref(false),
    canSyncKubernetesManifests: ref(false),
    canReadConfigPatches: ref(false),
    canManageConfigPatches: ref(false),
    canRebootMachines: ref(false),
    canRemoveMachines: ref(false),
    canManageClusterFeatures: ref(false),
  }

  const updatePermissions = async () => {
    const clusterPermissions = await ResourceService.Get({
      namespace: VirtualNamespace,
      type: ClusterPermissionsType,
      id: cluster.value,
    }, withRuntime(Runtime.Omni))

    result.canUpdateKubernetes.value = clusterPermissions?.spec?.can_update_kubernetes || false;
    result.canUpdateTalos.value = clusterPermissions?.spec?.can_update_talos || false;
    result.canDownloadKubeconfig.value = clusterPermissions?.spec?.can_download_kubeconfig || false;
    result.canDownloadTalosconfig.value = clusterPermissions?.spec?.can_download_talosconfig || false;
    result.canDownloadSupportBundle.value = clusterPermissions?.spec?.can_download_support_bundle || false;
    result.canAddClusterMachines.value = clusterPermissions?.spec?.can_add_machines || false;
    result.canRemoveClusterMachines.value = clusterPermissions?.spec?.can_remove_machines || false;
    result.canSyncKubernetesManifests.value = clusterPermissions?.spec?.can_sync_kubernetes_manifests || false;
    result.canReadConfigPatches.value = clusterPermissions?.spec?.can_read_config_patches || false;
    result.canManageConfigPatches.value = clusterPermissions?.spec?.can_manage_config_patches || false;
    result.canRebootMachines.value = clusterPermissions?.spec?.can_reboot_machines || false;
    result.canRemoveMachines.value = clusterPermissions?.spec?.can_remove_machines || false;
    result.canManageClusterFeatures.value = clusterPermissions?.spec?.can_manage_cluster_features || false;
  }

  onBeforeMount(updatePermissions);
  watch(cluster, updatePermissions);

  return result
};

export const canManageUsers: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_manage_users ?? false;
});

export const canReadClusters: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_read_clusters ?? false;
});

export const canReadMachines: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_read_machines ?? false;
});

export const canCreateClusters: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_create_clusters ?? false;
});

export const canRemoveMachines: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_remove_machines ?? false;
});

export const canReadMachineLogs: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_read_machine_logs ?? false;
});

export const canReadMachineConfigPatches: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_read_machine_config_patches ?? false;
});

export const canManageMachineConfigPatches: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_manage_machine_config_patches ?? false;
});

export const canManageBackupStore: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_manage_backup_store ?? false;
});

export const canAccessMaintenanceNodes: ComputedRef<boolean> = computed(() => {
  return permissions?.value?.spec?.can_access_maintenance_nodes ?? false;
});

export const loadCurrentUser = async () => {
  if (!currentUser.value) {
    await refreshCurrentUser();
  }

  if (!permissions.value) {
    await refreshPermissions();
  }
}

const refreshCurrentUser = async () => {
  if (!window.localStorage.getItem("identity")) {
    currentUser.value = undefined;
    return;
  }

  try {
    currentUser.value = await ResourceService.Get({
      namespace: VirtualNamespace,
      type: CurrentUserType,
      id: CurrentUserID,
    }, withRuntime(Runtime.Omni));
  } catch (e) {
    currentUser.value = undefined;
  }
}

const refreshPermissions = async () => {
  if (!currentUser.value) {
    permissions.value = undefined;
    return;
  }

  try {
    permissions.value = await ResourceService.Get({
      namespace: VirtualNamespace,
      type: PermissionsType,
      id: PermissionsID,
    }, withRuntime(Runtime.Omni));
  } catch (e) {
    permissions.value = undefined;
  }
}
