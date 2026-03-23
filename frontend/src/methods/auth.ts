// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import type { MaybeRefOrGetter } from 'vue'
import { computed, effectScope, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { AuthService } from '@/api/omni/auth/auth.pb'
import type { JoinTokenSpec } from '@/api/omni/specs/siderolink.pb'
import type {
  ClusterPermissionsSpec,
  CurrentUserSpec,
  PermissionsSpec,
} from '@/api/omni/specs/virtual.pb'
import { withRuntime } from '@/api/options'
import {
  AuthFlowQueryParam,
  ClusterPermissionsType,
  CurrentUserID,
  CurrentUserType,
  DefaultNamespace,
  FrontendAuthFlow,
  JoinTokenType,
  PermissionsID,
  PermissionsType,
  VirtualNamespace,
} from '@/api/resources'
import { AuthType, authType } from '@/methods'
import { useIdentity } from '@/methods/identity'
import { useKeys } from '@/methods/key'
import { redirectToURL } from '@/methods/navigate'
import { useResourceGet } from '@/methods/useResourceGet'

const authScope = effectScope(true)

const currentUser = authScope.run(() => {
  const { data } = useResourceGet<CurrentUserSpec>(() => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: CurrentUserType,
      id: CurrentUserID,
    },
  }))

  return data
})!

const permissions = authScope.run(() => {
  const { data } = useResourceGet<PermissionsSpec>(() => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: PermissionsType,
      id: PermissionsID,
    },
  }))

  const spec = computed(() => data.value?.spec)

  return {
    canAccessMaintenanceNodes: computed(() => spec.value?.can_access_maintenance_nodes ?? false),
    canCreateClusters: computed(() => spec.value?.can_create_clusters ?? false),
    canManageBackupStore: computed(() => spec.value?.can_manage_backup_store ?? false),
    canManageMachineConfigPatches: computed(
      () => spec.value?.can_manage_machine_config_patches ?? false,
    ),
    canManageUsers: computed(() => spec.value?.can_manage_users ?? false),
    canReadAuditLog: computed(() => spec.value?.can_read_audit_log ?? false),
    canReadClusters: computed(() => spec.value?.can_read_clusters ?? false),
    canReadMachineConfigPatches: computed(
      () => spec.value?.can_read_machine_config_patches ?? false,
    ),
    canReadMachineLogs: computed(() => spec.value?.can_read_machine_logs ?? false),
    canReadMachines: computed(() => spec.value?.can_read_machines ?? false),
    canRemoveMachines: computed(() => spec.value?.can_remove_machines ?? false),
  }
})!

export function useCurrentUser() {
  return currentUser
}

export function usePermissions() {
  return permissions
}

const clusterPermissionScopes: Record<string, ReturnType<typeof createClusterPermissions>> = {}

function createClusterPermissions(clusterName?: string) {
  return effectScope(true).run(() => {
    const { data } = useResourceGet<ClusterPermissionsSpec>({
      runtime: Runtime.Omni,
      resource: {
        namespace: VirtualNamespace,
        type: ClusterPermissionsType,
        id: clusterName,
      },
    })

    const spec = computed(() => data.value?.spec)

    return {
      canUpdateKubernetes: computed(() => spec.value?.can_update_kubernetes ?? false),
      canUpdateTalos: computed(() => spec.value?.can_update_talos ?? false),
      canDownloadKubeconfig: computed(() => spec.value?.can_download_kubeconfig ?? false),
      canDownloadTalosconfig: computed(() => spec.value?.can_download_talosconfig ?? false),
      canDownloadSupportBundle: computed(() => spec.value?.can_download_support_bundle ?? false),
      canAddClusterMachines: computed(() => spec.value?.can_add_machines ?? false),
      canRemoveClusterMachines: computed(() => spec.value?.can_remove_machines ?? false),
      canSyncKubernetesManifests: computed(
        () => spec.value?.can_sync_kubernetes_manifests ?? false,
      ),
      canReadConfigPatches: computed(() => spec.value?.can_read_config_patches ?? false),
      canManageConfigPatches: computed(() => spec.value?.can_manage_config_patches ?? false),
      canRebootMachines: computed(() => spec.value?.can_reboot_machines ?? false),
      canRemoveMachines: computed(() => spec.value?.can_remove_machines ?? false),
      canManageClusterFeatures: computed(() => spec.value?.can_manage_cluster_features ?? false),
    }
  })!
}

export function useClusterPermissions(cluster: MaybeRefOrGetter<string | undefined>) {
  const key = toValue(cluster) ?? '__NO_CLUSTER'
  return (clusterPermissionScopes[key] ??= createClusterPermissions(toValue(cluster)))
}

export const revokeJoinToken = async (tokenID: string) => {
  updateToken(tokenID, (token) => {
    token.spec.revoked = true
  })
}

export const unrevokeJoinToken = async (tokenID: string) => {
  updateToken(tokenID, (token) => {
    token.spec.revoked = false
  })
}

export const deleteJoinToken = async (tokenID: string) => {
  await ResourceService.Delete(
    {
      namespace: DefaultNamespace,
      type: JoinTokenType,
      id: tokenID,
    },
    withRuntime(Runtime.Omni),
  )
}

const updateToken = async (tokenID: string, update: (token: Resource<JoinTokenSpec>) => void) => {
  const token: Resource<JoinTokenSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: JoinTokenType,
      id: tokenID,
    },
    withRuntime(Runtime.Omni),
  )

  update(token)

  await ResourceService.Update(token, token.metadata.version, withRuntime(Runtime.Omni))
}

export function useLogout() {
  const auth0 = authType.value === AuthType.Auth0 ? useAuth0() : null
  const keys = useKeys()
  const identity = useIdentity()

  return async function () {
    if (keys.publicKeyID.value) {
      try {
        await AuthService.RevokePublicKey({ public_key_id: keys.publicKeyID.value })
      } catch (error) {
        // During a log out action being unauthenticated is fine
        if (error.code !== Code.UNAUTHENTICATED) throw error
      }
    }

    await auth0?.logout({
      logoutParams: {
        returnTo: window.location.origin,
      },
    })

    keys.clear()
    identity.clear()

    if (authType.value !== AuthType.Auth0) {
      redirectToURL(`/logout?${AuthFlowQueryParam}=${FrontendAuthFlow}`)
    }
  }
}
