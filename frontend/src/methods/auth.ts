// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import type { MaybeRefOrGetter } from 'vue'
import { computed, effectScope, nextTick, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
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
    canReadJoinTokens: computed(() => spec.value?.can_read_join_tokens ?? false),
    canManageJoinTokens: computed(() => spec.value?.can_manage_join_tokens ?? false),
    canReadInstallationMedia: computed(() => spec.value?.can_read_installation_media ?? false),
    canManageInstallationMedia: computed(() => spec.value?.can_manage_installation_media ?? false),
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

    return data
  })!
}

export function useClusterPermissions(cluster: MaybeRefOrGetter<string | undefined>) {
  const spec = computed(() => {
    const clusterName = toValue(cluster)
    const cacheKey = clusterName ?? '__NO_CLUSTER'

    return (clusterPermissionScopes[cacheKey] ??= createClusterPermissions(clusterName)).value?.spec
  })

  return {
    canUpdateKubernetes: computed(() => spec.value?.can_update_kubernetes ?? false),
    canUpdateTalos: computed(() => spec.value?.can_update_talos ?? false),
    canDownloadKubeconfig: computed(() => spec.value?.can_download_kubeconfig ?? false),
    canDownloadTalosconfig: computed(() => spec.value?.can_download_talosconfig ?? false),
    canDownloadSupportBundle: computed(() => spec.value?.can_download_support_bundle ?? false),
    canAddClusterMachines: computed(() => spec.value?.can_add_machines ?? false),
    canRemoveClusterMachines: computed(() => spec.value?.can_remove_machines ?? false),
    canSyncKubernetesManifests: computed(() => spec.value?.can_sync_kubernetes_manifests ?? false),
    canReadConfigPatches: computed(() => spec.value?.can_read_config_patches ?? false),
    canManageConfigPatches: computed(() => spec.value?.can_manage_config_patches ?? false),
    canRebootMachines: computed(() => spec.value?.can_reboot_machines ?? false),
    canRemoveMachines: computed(() => spec.value?.can_remove_machines ?? false),
    canManageClusterFeatures: computed(() => spec.value?.can_manage_cluster_features ?? false),
    canReadMachineConfig: computed(() => spec.value?.can_read_machine_config ?? false),
    canManageMachineConfig: computed(() => spec.value?.can_manage_machine_config ?? false),
    canReadKernelArgs: computed(() => spec.value?.can_read_kernel_args ?? false),
    canManageKernelArgs: computed(() => spec.value?.can_manage_kernel_args ?? false),
    canReadMachinePendingUpdates: computed(
      () => spec.value?.can_read_machine_pending_updates ?? false,
    ),
  }
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

const redirectToURL = (url: string) => {
  if (window.top) {
    window.top.location.href = url
  } else {
    window.location.href = url
  }
}

export function useLogout() {
  const auth0 = authType.value === AuthType.Auth0 ? useAuth0() : null
  const keys = useKeys()
  const identity = useIdentity()

  return async function () {
    if (keys.publicKeyID.value) {
      try {
        await AuthService.RevokePublicKey({ public_key_id: keys.publicKeyID.value })
      } catch (e) {
        // During a log out action being unauthenticated is fine
        if (!(e instanceof RequestError) || e.code !== Code.UNAUTHENTICATED) throw e
      }
    }

    keys.clear()
    identity.clear()

    // Wait for storages to be set
    await nextTick()

    if (auth0) {
      await auth0.logout({
        logoutParams: {
          returnTo: window.location.origin,
        },
      })
    } else {
      redirectToURL(`/logout?${AuthFlowQueryParam}=${FrontendAuthFlow}`)
    }
  }
}
