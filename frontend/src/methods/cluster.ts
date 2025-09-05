// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { isEqual } from 'lodash'
import * as semver from 'semver'
import type { Ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { DeleteRequest } from '@/api/omni/resources/resources.pb'
import type {
  ClusterSpec,
  EtcdManualBackupSpec,
  KubernetesUpgradeStatusSpec,
  MachineSetNodeSpec,
  NodeForceDestroyRequestSpec,
  TalosUpgradeStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { withRuntime, withSelectors, withTimeout } from '@/api/options'
import {
  ClusterLocked,
  ClusterType,
  ConfigPatchType,
  DefaultNamespace,
  EphemeralNamespace,
  EtcdManualBackupType,
  KubernetesUpgradeStatusType,
  LabelCluster,
  LabelMachine,
  LabelMachineSet,
  MachineSetNodeType,
  MachineSetType,
  NodeForceDestroyRequestType,
  TalosUpgradeStatusType,
} from '@/api/resources'
import type { Metadata } from '@/api/v1alpha1/resource.pb'
import { embeddedDiscoveryServiceFeatureAvailable } from '@/methods/features'
import { parseLabels } from '@/methods/labels'
import { controlPlaneMachineSetId } from '@/methods/machineset'

import { isoNow } from './time'

export type MachineConfig = {
  role?: string
  configPatch: string
  installDisk?: string
  id?: string
}

type MachineSetNode = {
  metadata: Metadata
  spec: MachineSetNodeSpec
}

const createResources = async (resources: Resource[]) => {
  for (const resource of resources) {
    try {
      await ResourceService.Create(resource, withRuntime(Runtime.Omni))
    } catch (e) {
      throw new ClusterCommandError(
        `Failed to Create ${resource.metadata.type}(${resource.metadata.namespace}/${resource.metadata.id}})`,
        `The resource creation failed with the error: ${e.message}`,
      )
    }
  }
}

const updateResource = async <T extends Resource>(
  metadata: Metadata,
  updater: (spec: T) => void,
) => {
  const resource: T = await ResourceService.Get<T>(
    {
      type: metadata.type,
      namespace: metadata.namespace,
      id: metadata.id,
    },
    withRuntime(Runtime.Omni),
  )

  updater(resource)

  await ResourceService.Update<T>(resource, resource.metadata.version, withRuntime(Runtime.Omni))
}

const timeout = 10 * 60 * 1000 // 10 minutes

export const destroyResources = async (resources: DeleteRequest[], phase?: Ref<string>) => {
  for (const request of resources) {
    try {
      if (phase) {
        phase.value = `Destroying resource ${request.type}(${request.namespace}/${request.id}`
      }

      await ResourceService.Delete(request, withRuntime(Runtime.Omni), withTimeout(timeout))
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        throw e
      }
    }
  }
}

export class ClusterCommandError extends Error {
  private title: string

  constructor(title: string, msg: string) {
    super(msg)

    this.title = title

    Object.setPrototypeOf(this, ClusterCommandError.prototype)
  }

  get errorNotification(): { title: string; details: string } {
    return {
      title: this.title,
      details: this.message,
    }
  }
}

export const clusterSync = async (resources: Resource[], old?: Resource[]) => {
  let toCreate = resources
  const toUpdate: Resource[] = []
  const toDestroy: DeleteRequest[] = []

  if (old) {
    const resourcesMap: Record<string, Resource> = {}
    old.forEach((item) => {
      resourcesMap[`${item.metadata.type}/${item.metadata.id}`] = item
    })

    toCreate = []
    resources.forEach((item) => {
      const id = `${item.metadata.type}/${item.metadata.id}`
      const existing = resourcesMap[id]

      delete resourcesMap[id]

      if (!existing) {
        toCreate.push(item)

        return
      }

      if (
        !isEqual(existing.metadata.labels ?? {}, item.metadata.labels ?? {}) ||
        !isEqual(existing.metadata.annotations ?? {}, item.metadata.annotations ?? {}) ||
        !isEqual(existing.spec, item.spec)
      ) {
        toUpdate.push(item)

        return
      }
    })

    for (const key in resourcesMap) {
      const md = resourcesMap[key].metadata
      toDestroy.push({
        id: md.id,
        namespace: md.namespace,
        type: md.type,
      })
    }
  }

  await createResources(toCreate)

  for (const res of toUpdate) {
    await updateResource(res.metadata, (r: Resource) => {
      r.metadata.annotations = res.metadata.annotations
      r.metadata.labels = res.metadata.labels
      r.spec = res.spec
    })
  }

  for (const res of toDestroy) {
    try {
      switch (res.type) {
        case MachineSetType:
        case ClusterType:
          await ResourceService.Teardown(res, withRuntime(Runtime.Omni))

          break
        default:
          await ResourceService.Delete(res, withRuntime(Runtime.Omni))
      }
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        throw e
      }
    }
  }
}

export const getMachineConfigPatchesToDelete = async (machineID: string) => {
  const resources: DeleteRequest[] = []

  try {
    const patches: Resource[] = await ResourceService.List(
      {
        namespace: DefaultNamespace,
        type: ConfigPatchType,
      },
      withRuntime(Runtime.Omni),
      withSelectors([`${LabelMachine}=${machineID}`]),
    )

    for (const patch of patches) {
      resources.push({
        id: patch.metadata.id!,
        namespace: patch.metadata.namespace!,
        type: patch.metadata.type!,
      })
    }
  } catch {
    throw new ClusterCommandError(
      `Failed to Destroy Machine ${machineID}`,
      'Failed to fetch machine patches for the node',
    )
  }

  return resources
}

export const destroyNodes = async (clusterName: string, ids: string[], force: boolean) => {
  const machineSetNodes: Record<string, MachineSetNode> = {}
  let controlPlanes = 0

  try {
    const resp: Resource[] = await ResourceService.List(
      {
        namespace: DefaultNamespace,
        type: MachineSetNodeType,
      },
      withRuntime(Runtime.Omni),
      withSelectors([`${LabelCluster}=${clusterName}`]),
    )

    for (const item of resp) {
      machineSetNodes[item.metadata.id!] = item

      if (!item.metadata.labels) {
        item.metadata.labels = {}
      }

      if (item.metadata.labels[LabelMachineSet] === controlPlaneMachineSetId(clusterName)) {
        controlPlanes++
      }
    }
  } catch (e) {
    throw new ClusterCommandError(
      'Failed to Destroy Node',
      `Failed to fetch the machine set nodes: ${e.message}`,
    )
  }

  const resources: DeleteRequest[] = []

  for (const id of ids) {
    if (
      (machineSetNodes[id]?.metadata?.labels || {})[LabelMachineSet] ===
        controlPlaneMachineSetId(clusterName) &&
      controlPlanes === 1
    ) {
      throw new ClusterCommandError(
        `Failed to Destroy Node ${id}`,
        'The cluster should have at least one control plane running',
      )
    }

    if (force) {
      // if this is a force-delete, MachineSetNode might already be gone, and it is ok
      try {
        await ResourceService.Get<Resource<NodeForceDestroyRequestSpec>>(
          {
            type: NodeForceDestroyRequestType,
            namespace: DefaultNamespace,
            id: id,
          },
          withRuntime(Runtime.Omni),
        )
      } catch (e) {
        if (e.code !== Code.NOT_FOUND) {
          throw e
        }

        const forceDeleteRequest: Resource<NodeForceDestroyRequestSpec> = {
          metadata: {
            id: id,
            namespace: DefaultNamespace,
            type: NodeForceDestroyRequestType,
          },
          spec: {},
        }

        await ResourceService.Create(forceDeleteRequest, withRuntime(Runtime.Omni))
      }
    } else if (!machineSetNodes[id]?.metadata) {
      // if this is not a force-delete, and the node is not found, it is an error
      throw new ClusterCommandError(
        `Failed to Destroy Node ${id}`,
        'The node with such id is not part of the cluster',
      )
    }

    if (machineSetNodes[id]) {
      const md = machineSetNodes[id]?.metadata
      resources.push({
        id: md.id!,
        namespace: md.namespace!,
        type: md.type!,
      })
    }
  }

  await destroyResources(resources)
}

export const restoreNode = async (clusterMachine: Resource) => {
  const clusterName = clusterMachine.metadata.labels?.[LabelCluster]
  const machineSetID = clusterMachine.metadata.labels?.[LabelMachineSet]

  try {
    if (!clusterName || !machineSetID) {
      throw new ClusterCommandError(
        'Failed To Rollback Machine Set Node',
        "Corresponding cluster machine doesn't have cluster name or machine set labels",
      )
    }

    await createResources([
      {
        metadata: {
          namespace: DefaultNamespace,
          type: MachineSetNodeType,
          id: clusterMachine.metadata.id,
          labels: {
            [LabelCluster]: clusterName,
            [LabelMachineSet]: machineSetID,
          },
        },
        spec: {},
      },
    ])
  } catch (e) {
    throw new ClusterCommandError(
      'Failed To Rollback Machine Set Node',
      `Failed to restore machine set node ${clusterMachine.metadata.id} for ${clusterName}: ${e.message}`,
    )
  }
}

export const clusterDestroy = async (clusterName: string) => {
  await ResourceService.Teardown(
    {
      namespace: DefaultNamespace,
      type: ClusterType,
      id: clusterName,
    },
    withRuntime(Runtime.Omni),
    withTimeout(timeout),
  )
}

export const nextAvailableClusterName = async (prefix: string) => {
  const clusters: Resource<ClusterSpec>[] = await ResourceService.List(
    {
      namespace: DefaultNamespace,
      type: ClusterType,
    },
    withRuntime(Runtime.Omni),
  )

  const nameSet = new Set(clusters.map((item) => item.metadata.id!))

  if (!nameSet.has(prefix)) {
    return prefix
  }

  let lastNum = 0

  while (nameSet.has(`${prefix}-${lastNum + 1}`)) {
    lastNum++
  }

  return `${prefix}-${lastNum + 1}`
}

export const upgradeKubernetes = async (clusterName: string, version: string) => {
  await updateResource<Resource<ClusterSpec>>(
    {
      namespace: DefaultNamespace,
      type: ClusterType,
      id: clusterName,
    },
    (res: Resource<ClusterSpec>) => {
      res.spec.kubernetes_version = version
    },
  )
}

export const updateTalos = async (clusterName: string, version: string) => {
  await updateResource<Resource<ClusterSpec>>(
    {
      namespace: DefaultNamespace,
      type: ClusterType,
      id: clusterName,
    },
    (res: Resource<ClusterSpec>) => {
      res.spec.talos_version = version
    },
  )
}

export const revertKubernetesUpgrade = async (clusterName: string) => {
  const upgradeStatus: Resource<TalosUpgradeStatusSpec | KubernetesUpgradeStatusSpec> =
    await ResourceService.Get(
      {
        namespace: DefaultNamespace,
        type: KubernetesUpgradeStatusType,
        id: clusterName,
      },
      withRuntime(Runtime.Omni),
    )

  await upgradeKubernetes(clusterName, upgradeStatus.spec.last_upgrade_version!)
}

export const revertTalosUpgrade = async (clusterName: string) => {
  const upgradeStatus: Resource<TalosUpgradeStatusSpec | KubernetesUpgradeStatusSpec> =
    await ResourceService.Get(
      {
        namespace: DefaultNamespace,
        type: TalosUpgradeStatusType,
        id: clusterName,
      },
      withRuntime(Runtime.Omni),
    )

  await updateTalos(clusterName, upgradeStatus.spec.last_upgrade_version!)
}

export const addClusterLabels = async (clusterID: string, ...labels: string[]) => {
  const resource: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterID,
    },
    withRuntime(Runtime.Omni),
  )

  resource.metadata.labels = {
    ...parseLabels(...labels),
    ...resource.metadata.labels,
  }

  await ResourceService.Update(resource, resource.metadata.version, withRuntime(Runtime.Omni))
}

export const removeClusterLabels = async (clusterID: string, ...keys: string[]) => {
  const resource: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterID,
    },
    withRuntime(Runtime.Omni),
  )

  if (!resource.metadata.labels) {
    return
  }

  for (const key of keys) {
    delete resource.metadata.labels[key]
  }

  await ResourceService.Update(resource, undefined, withRuntime(Runtime.Omni))
}

export const setClusterWorkloadProxy = async (clusterID: string, enabled: boolean) => {
  const resource: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterID,
    },
    withRuntime(Runtime.Omni),
  )

  if (!resource.spec.features) {
    resource.spec.features = {}
  }

  resource.spec.features.enable_workload_proxy = enabled

  await ResourceService.Update(resource, resource.metadata.version, withRuntime(Runtime.Omni))
}

export const setUseEmbeddedDiscoveryService = async (clusterID: string, enabled: boolean) => {
  const resource: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterID,
    },
    withRuntime(Runtime.Omni),
  )

  if (!resource.spec.features) {
    resource.spec.features = {}
  }

  resource.spec.features.use_embedded_discovery_service = enabled

  await ResourceService.Update(resource, resource.metadata.version, withRuntime(Runtime.Omni))
}

export const setClusterEtcdBackupsConfig = async (clusterID: string, spec: ClusterSpec) => {
  const resource: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterID,
    },
    withRuntime(Runtime.Omni),
  )

  if (isEqual(resource.spec.backup_configuration, spec.backup_configuration)) {
    return
  }

  resource.spec.backup_configuration = spec.backup_configuration

  await ResourceService.Update(resource, resource.metadata.version, withRuntime(Runtime.Omni))
}

export const triggerEtcdBackup = async (clusterID: string) => {
  const metadata = {
    type: EtcdManualBackupType,
    namespace: EphemeralNamespace,
    id: clusterID,
  }

  let exists = false

  let manualBackup: Resource<EtcdManualBackupSpec>
  try {
    manualBackup = await ResourceService.Get(metadata, withRuntime(Runtime.Omni))

    exists = true
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e
    }

    manualBackup = {
      metadata,
      spec: {},
    }
  }

  manualBackup.spec.backup_at = isoNow()

  if (!exists) {
    return await ResourceService.Create(manualBackup, withRuntime(Runtime.Omni))
  }

  return await ResourceService.Update(manualBackup, undefined, withRuntime(Runtime.Omni))
}

export const embeddedDiscoveryServiceAvailable = async (
  talosVersion?: string,
): Promise<boolean> => {
  if (!talosVersion) {
    return false
  }

  if (semver.compare(talosVersion, 'v1.5.0') < 0) {
    return false
  }

  return await embeddedDiscoveryServiceFeatureAvailable()
}

export const updateClusterLock = async (id: string, locked: boolean) => {
  const cluster = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: ClusterType,
      id: id,
    },
    withRuntime(Runtime.Omni),
  )

  if (!cluster.metadata.annotations) cluster.metadata.annotations = {}

  if (locked) {
    cluster.metadata.annotations[ClusterLocked] = ''
  } else {
    delete cluster.metadata.annotations[ClusterLocked]
  }

  await ResourceService.Update(cluster, undefined, withRuntime(Runtime.Omni))
}
