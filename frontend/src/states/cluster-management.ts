// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// MachineSet is the state used in cluster creation flow.

import yaml from 'js-yaml'
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type {
  ClusterSpec,
  ConfigPatchSpec,
  EtcdBackupConf,
  ExtensionsConfigurationSpec,
  MachineSetNodeSpec,
  MachineSetSpec,
  MachineSetSpecBootstrapSpec,
  MachineSetSpecUpdateStrategyConfig,
} from '@/api/omni/specs/omni.pb'
import {
  MachineSetSpecMachineAllocationType,
  MachineSetSpecUpdateStrategy,
} from '@/api/omni/specs/omni.pb'
import { withRuntime, withSelectors } from '@/api/options'
import {
  ClusterType,
  ConfigPatchName,
  ConfigPatchType,
  DefaultKubernetesVersion,
  DefaultNamespace,
  DefaultTalosVersion,
  ExtensionsConfigurationType,
  LabelCluster,
  LabelClusterMachine,
  LabelControlPlaneRole,
  LabelMachineSet,
  LabelManagedByMachineSetNodeController,
  LabelSystemPatch,
  LabelWorkerRole,
  MachineSetNodeType,
  MachineSetType,
  PatchBaseWeightCluster,
  PatchBaseWeightClusterMachine,
  PatchBaseWeightMachineSet,
} from '@/api/resources'
import { parseLabels } from '@/methods/labels'
import { controlPlaneMachineSetId, defaultWorkersMachineSetId } from '@/methods/machineset'

export const typesOrder = {
  [ClusterType]: 4,
  [ConfigPatchType]: 3,
  [MachineSetType]: 2,
  [MachineSetNodeType]: 1,
}

const dec2hex = (dec: number) => {
  return dec.toString(16).padStart(2, '0')
}

const generateID = (len: number) => {
  const arr = new Uint8Array((len || 40) / 2)
  window.crypto.getRandomValues(arr)

  return Array.from(arr, dec2hex).join('')
}

export const unlimited = 'unlimited'

export enum PatchID {
  Default = '',
  Untaint = 'untaint',
  InstallDisk = 'install-disk',
}

const colors = [
  'red',
  'orange',
  'violet',
  'yellow',
  'cyan',
  'blue1',
  'blue2',
  'blue3',
  'light1',
  'light2',
  'light3',
  'light4',
  'light5',
  'light6',
  'light7',
]

// Keeps the configuration for the machine set.
export interface MachineSet {
  id: string
  name: string
  role: string
  color: string
  machineAllocation?: {
    name: string
    size: number | 'unlimited'
  }
  machines: Record<string, MachineSetNode>
  patches: Record<string, ConfigPatch>
  bootstrapSpec?: MachineSetSpecBootstrapSpec
  updateStrategy?: {
    type?: MachineSetSpecUpdateStrategy
    config?: MachineSetSpecUpdateStrategyConfig
  }
  deleteStrategy?: {
    type?: MachineSetSpecUpdateStrategy
    config?: MachineSetSpecUpdateStrategyConfig
  }
  idFunc: (cluster: string) => string
}

export interface MachineSetNode {
  patches: Record<string, ConfigPatch>
  systemExtensions?: string[]
}

export type Cluster = {
  labels?: Record<string, string>
  annotations?: Record<string, string>
  name?: string
  talosVersion?: string
  kubernetesVersion?: string
  features: {
    encryptDisks?: boolean
    enableWorkloadProxy?: boolean
    useEmbeddedDiscoveryService?: boolean
  }
  systemExtensions?: string[]
  etcdBackupConfig?: EtcdBackupConf
  patches: Record<string, ConfigPatch>
}

export type ConfigPatch = {
  data: string
  weight: number
  systemPatch?: boolean
  nameAnnotation?: string
}

export class State {
  public machineSets: MachineSet[] = []

  public cluster: Cluster = {
    talosVersion: DefaultTalosVersion,
    kubernetesVersion: DefaultKubernetesVersion,
    patches: {},
    features: {},
  }

  public index: number = 1

  public baseResources?: Resource[]

  constructor() {
    this.machineSets = [
      {
        id: 'CP',
        name: 'control planes',
        role: LabelControlPlaneRole,
        color: 'green',
        machines: {},
        idFunc: controlPlaneMachineSetId,
        patches: {},
      },
      {
        id: 'W0',
        name: 'main worker pool',
        role: LabelWorkerRole,
        color: 'red',
        machines: {},
        idFunc: defaultWorkersMachineSetId,
        patches: {},
      },
    ]
  }

  public get controlPlanesCount() {
    return this.machinesOfKind(LabelControlPlaneRole)
  }

  public get workersCount() {
    return this.machinesOfKind(LabelWorkerRole)
  }

  setClusterName(name: string) {
    this.cluster.name = name
  }

  addClusterLabels(labels: string[]) {
    this.cluster.labels = {
      ...this.cluster.labels,
      ...parseLabels(...labels),
    }
  }

  removeClusterLabels(keys: string[]) {
    if (!this.cluster.labels) {
      return
    }

    for (const key of keys) {
      delete this.cluster.labels[key]
    }
  }

  addMachineSet(role: string): MachineSet {
    const id = generateID(6)

    this.machineSets.push({
      id: `W${this.index}`,
      name: `workers-${id}`,
      role: role,
      color: colors[this.index % colors.length],
      machines: {},
      patches: {},
      idFunc: (cluster: string) => `${cluster}-w${id}`,
    })

    this.index++

    return this.machineSets[this.machineSets.length - 1]
  }

  removeMachineSet(index: number) {
    if (index < 2) {
      throw new Error('removing primary machine sets is not allowed')
    }

    this.machineSets.splice(index, 1)
  }

  setMachine(index: number, id: string, machineSetNode: MachineSetNode) {
    this.machineSets[index].machines[id] = machineSetNode
  }

  removeMachine(index: number, id: string) {
    if (index < this.machineSets.length) {
      delete this.machineSets[index].machines[id]
    }
  }

  untaintSingleNode(): boolean {
    let nodes = 0

    for (const ms of this.machineSets) {
      if (ms.machineAllocation?.size === 'unlimited') {
        return false
      }

      const machineSetSize = (ms.machineAllocation?.size ?? NaN) as number

      nodes += !isNaN(machineSetSize) ? machineSetSize : Object.keys(ms.machines).length
    }

    // not a single node cluster
    if (nodes !== 1) {
      return false
    }

    // get all patches for cluster, control plane machine set and control plane machines
    const cp = this.controlPlanes()
    const patches = [
      ...Object.values(this.cluster.patches),
      ...Object.values(cp.patches),
      ...Object.values(cp.machines[Object.keys(cp.machines)[0]]?.patches ?? {}),
    ]

    // already enabled in one of the config patches
    for (const patch of patches) {
      if (this.checkSchedulingEnabled(patch.data)) {
        return false
      }
    }

    return true
  }

  machineSetID(index: number): string {
    if (!this.cluster.name || index > this.machineSets.length) {
      throw new Error(`can not get the id of the machine set at index ${index}`)
    }

    return this.machineSets[index].idFunc(this.cluster.name)
  }

  controlPlanes(): MachineSet {
    return this.machineSets[0]
  }

  resources(): Resource[] {
    if (!this.cluster.name) {
      throw new Error('cluster name is not set')
    }

    const cluster: Resource<ClusterSpec> = {
      metadata: {
        namespace: DefaultNamespace,
        type: ClusterType,
        id: this.cluster.name,
        labels: this.cluster.labels,
        annotations: this.cluster.annotations,
      },
      spec: {
        talos_version: this.cluster.talosVersion,
        kubernetes_version: this.cluster.kubernetesVersion,
        features: {},
      },
    }

    if (this.cluster.features.enableWorkloadProxy) {
      cluster.spec.features = {
        ...cluster.spec.features,
        enable_workload_proxy: this.cluster.features.enableWorkloadProxy,
      }
    }

    if (this.cluster.features.useEmbeddedDiscoveryService) {
      cluster.spec.features = {
        ...cluster.spec.features,
        use_embedded_discovery_service: this.cluster.features.useEmbeddedDiscoveryService,
      }
    }

    if (this.cluster.features.encryptDisks) {
      cluster.spec.features = {
        ...cluster.spec.features,
        disk_encryption: this.cluster.features.encryptDisks,
      }
    }

    if (
      this.cluster.etcdBackupConfig &&
      this.cluster.etcdBackupConfig.enabled &&
      this.cluster.etcdBackupConfig.interval
    ) {
      cluster.spec.backup_configuration = {
        enabled: this.cluster.etcdBackupConfig.enabled,
        interval: this.cluster.etcdBackupConfig.interval,
      }
    } else {
      cluster.spec.backup_configuration = undefined
    }

    let resources: Resource[] = [cluster]

    if (this.cluster.systemExtensions) {
      const systemExtensions = this.cluster.systemExtensions

      if (systemExtensions) {
        resources.push({
          metadata: {
            id: `schematic-${this.cluster.name}`,
            namespace: DefaultNamespace,
            type: ExtensionsConfigurationType,
            labels: {
              [LabelCluster]: this.cluster.name,
            },
          },
          spec: {
            extensions: systemExtensions,
          },
        })
      }
    }

    resources.push(
      ...this.getPatches(
        this.cluster.patches,
        {
          [LabelCluster]: this.cluster.name,
        },
        this.cluster.name,
      ),
    )

    const machineSets = [...this.machineSets].sort((a: MachineSet, b: MachineSet) => {
      if (a.machineAllocation && !b.machineAllocation) {
        return 1
      }

      if (b.machineAllocation && !a.machineAllocation) {
        return -1
      }

      return 0
    })

    for (const machineSet of machineSets) {
      const machineSetID = machineSet.idFunc(this.cluster.name)

      const ms: Resource<MachineSetSpec> = {
        metadata: {
          id: machineSetID,
          namespace: DefaultNamespace,
          type: MachineSetType,
          labels: {
            [LabelCluster]: this.cluster.name,
            [machineSet.role]: '',
          },
        },
        spec: {
          update_strategy: MachineSetSpecUpdateStrategy.Rolling,
        },
      }

      // preserve the bootstrap spec as-is if it was originally set, as it is immutable after creation
      ms.spec.bootstrap_spec = machineSet.bootstrapSpec

      if (machineSet.role === LabelWorkerRole) {
        ms.spec.update_strategy =
          machineSet.updateStrategy?.type ?? MachineSetSpecUpdateStrategy.Rolling

        if (machineSet.updateStrategy?.config) {
          ms.spec.update_strategy_config = machineSet.updateStrategy?.config
        }

        if (machineSet.deleteStrategy?.type !== undefined) {
          ms.spec.delete_strategy = machineSet.deleteStrategy?.type
        }

        if (machineSet.deleteStrategy?.config) {
          ms.spec.delete_strategy_config = machineSet.deleteStrategy?.config
        }
      }

      ms.spec.machine_class = undefined
      ms.spec.machine_allocation = undefined

      if (machineSet.machineAllocation) {
        ms.spec.machine_allocation = {
          name: machineSet.machineAllocation.name,
        }

        switch (machineSet.machineAllocation.size) {
          case unlimited:
            ms.spec.machine_allocation.allocation_type =
              MachineSetSpecMachineAllocationType.Unlimited

            break
          default:
            ms.spec.machine_allocation.machine_count = machineSet.machineAllocation.size as number

            break
        }
      }

      for (const id in machineSet.machines) {
        const msn: Resource<MachineSetNodeSpec> = {
          metadata: {
            id: id,
            namespace: DefaultNamespace,
            type: MachineSetNodeType,
            labels: {
              [LabelCluster]: this.cluster.name,
              [machineSet.role]: '',
              [LabelMachineSet]: machineSetID,
            },
          },
          spec: {},
        }

        const systemExtensions = machineSet.machines[id].systemExtensions

        if (systemExtensions) {
          resources.push({
            metadata: {
              id: `schematic-${id}`,
              namespace: DefaultNamespace,
              type: ExtensionsConfigurationType,
              labels: {
                [LabelCluster]: this.cluster.name,
                [LabelClusterMachine]: id,
              },
            },
            spec: {
              extensions: systemExtensions,
            },
          })
        }

        if (!machineSet.machineAllocation) resources.push(msn)

        resources.push(
          ...this.getPatches(machineSet.machines[id].patches, {
            [LabelCluster]: this.cluster.name,
            [LabelClusterMachine]: msn.metadata.id!,
          }),
        )
      }

      resources.push(ms)

      resources.push(
        ...this.getPatches(
          machineSet.patches,
          {
            [LabelCluster]: this.cluster.name,
            [LabelMachineSet]: ms.metadata.id!,
          },
          ms.metadata.id,
        ),
      )
    }

    resources = resources.sort((a: Resource, b: Resource) => {
      type TypeKey = keyof typeof typesOrder

      if (typesOrder[a.metadata.type! as TypeKey] > typesOrder[b.metadata.type! as TypeKey]) {
        return -1
      } else if (
        typesOrder[a.metadata.type! as TypeKey] < typesOrder[b.metadata.type! as TypeKey]
      ) {
        return 1
      }

      return 0
    })

    if (!this.baseResources) {
      return resources
    }

    const baseResources: Record<string, Record<string, Resource>> = {}

    for (const r of this.baseResources) {
      baseResources[r.metadata.type!] = baseResources[r.metadata.type!] || {}

      baseResources[r.metadata.type!][r.metadata.id!] = r
    }

    return resources.map((res) => {
      const base = baseResources[res.metadata.type!]?.[res.metadata.id!]
      if (!base) return res

      return {
        metadata: {
          ...base.metadata,
          ...res.metadata,
        },
        spec: {
          ...base.spec,
          ...res.spec,
        },
      }
    })
  }

  private getPatches(
    patches: Record<string, ConfigPatch>,
    labels: Record<string, string>,
    prefix?: string,
  ): Resource<ConfigPatchSpec>[] {
    const res: Resource<ConfigPatchSpec>[] = []

    for (const key in patches) {
      const systemLabels: Record<string, string> = {}
      const patch = patches[key]

      if (patch.systemPatch) {
        systemLabels[LabelSystemPatch] = ''
      }

      const parts: string[] = []
      if (prefix) {
        parts.push(prefix)
      }

      if (key !== '') {
        parts.push(key)
      }

      const id = formatPatchID(parts.join('-'), patch.weight)

      const patchResource: Resource<ConfigPatchSpec> = {
        metadata: {
          id: id,
          type: ConfigPatchType,
          labels: {
            ...systemLabels,
            ...labels,
          },
          namespace: DefaultNamespace,
        },
        spec: {
          data: patch.data,
        },
      }

      if (patch.nameAnnotation) {
        patchResource.metadata.annotations = {
          [ConfigPatchName]: patch.nameAnnotation,
        }
      }

      res.push(patchResource)
    }

    return res
  }

  private checkSchedulingEnabled(data: string) {
    const loaded = yaml.load(data) as
      | { cluster?: { allowSchedulingOnControlPlanes?: boolean } }
      | undefined
      | null

    return loaded?.cluster?.allowSchedulingOnControlPlanes
  }

  private machinesOfKind(kind: string): number | string {
    let count = 0

    for (const ms of this.machineSets) {
      if (ms.role !== kind) {
        continue
      }

      if (ms.machineAllocation) {
        if (ms.machineAllocation.size === 'unlimited') {
          return `All From Class "${ms.machineAllocation.name}"`
        }

        count += ms.machineAllocation.size as number
      } else {
        count += Object.keys(ms.machines).length
      }
    }

    return count
  }
}

export const state = ref<State>(new State())

export const initState = () => {
  state.value = new State()

  return state
}

const formatPatchID = (key: string, weight: number) =>
  `${weight.toString().padStart(3, '0')}-${key}`

export const populateExisting = async (clusterName: string) => {
  initState()

  const resources: Resource[] = []

  // list all cluster config patches
  const patches: Resource<ConfigPatch>[] = await ResourceService.List(
    {
      type: ConfigPatchType,
      namespace: DefaultNamespace,
    },
    withRuntime(Runtime.Omni),
    withSelectors([`${LabelCluster}=${clusterName}`]),
  )

  const patchesByID: Record<string, Resource<ConfigPatchSpec>> = {}

  for (const patch of patches) {
    patchesByID[patch.metadata.id!] = patch
  }

  const systemExtensions: Resource<ExtensionsConfigurationSpec>[] = await ResourceService.List(
    {
      type: ExtensionsConfigurationType,
      namespace: DefaultNamespace,
    },
    withRuntime(Runtime.Omni),
    withSelectors([`${LabelCluster}=${clusterName}`]),
  )

  const systemExtensionsByID: Record<string, Resource<ExtensionsConfigurationSpec>> = {}

  for (const configuration of systemExtensions) {
    systemExtensionsByID[configuration.metadata.id!] = configuration

    resources.push(configuration)
  }

  // get cluster
  const cluster: Resource<ClusterSpec> = await ResourceService.Get(
    {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: clusterName,
    },
    withRuntime(Runtime.Omni),
  )

  resources.push(cluster)

  state.value.cluster = {
    name: clusterName,
    kubernetesVersion: cluster.spec.kubernetes_version,
    talosVersion: cluster.spec.talos_version,
    labels: cluster.metadata.labels ?? {},
    annotations: cluster.metadata.annotations ?? {},
    patches: {},
    features: {
      enableWorkloadProxy: cluster.spec.features?.enable_workload_proxy,
      encryptDisks: cluster.spec.features?.disk_encryption,
      useEmbeddedDiscoveryService: cluster.spec.features?.use_embedded_discovery_service,
    },
    etcdBackupConfig: {
      enabled: cluster.spec.backup_configuration?.enabled,
      interval: cluster.spec.backup_configuration?.interval,
    },
    systemExtensions: systemExtensionsByID[`schematic-${cluster.metadata.id}`]?.spec?.extensions,
  }

  const clusterPatch = patchesByID[formatPatchID(clusterName, PatchBaseWeightCluster)]

  if (clusterPatch) {
    state.value.cluster.patches[PatchID.Default] = {
      data: clusterPatch.spec.data!,
      weight: PatchBaseWeightCluster,
    }

    resources.push(clusterPatch)
  }

  // list machine sets
  const machineSets: Resource<MachineSetSpec>[] = await ResourceService.List(
    {
      type: MachineSetType,
      namespace: DefaultNamespace,
    },
    withRuntime(Runtime.Omni),
    withSelectors([`${LabelCluster}=${clusterName}`]),
  )

  state.value.index = 0

  const machineSetList: MachineSet[] = []

  const machineSetsByID: Record<string, MachineSet> = {}

  machineSets.sort((a, b) => {
    if (a.metadata.id === controlPlaneMachineSetId(clusterName)) {
      return -1
    }

    if (
      a.metadata.id === defaultWorkersMachineSetId(clusterName) &&
      b.metadata.id !== controlPlaneMachineSetId(clusterName)
    ) {
      return -1
    }

    return 0
  })

  for (let i = 0; i < machineSets.length; i++) {
    const ms = machineSets[i]
    if (ms.metadata.phase === 'tearingDown') {
      continue
    }

    const isCP = ms.metadata.labels?.[LabelControlPlaneRole] !== undefined

    const machineSet: MachineSet = {
      color: isCP ? 'green' : colors[state.value.index % colors.length],
      id: isCP ? 'CP' : `W${state.value.index}`,
      name: ms.metadata.id!,
      idFunc: () => ms.metadata.id!,
      role: isCP ? LabelControlPlaneRole : LabelWorkerRole,
      bootstrapSpec: ms.spec.bootstrap_spec,
      machines: {},
      patches: {},
    }

    const allocationCfg = ms.spec.machine_class ?? ms.spec.machine_allocation

    if (allocationCfg) {
      machineSet.machineAllocation = {
        name: allocationCfg.name!,
        size:
          allocationCfg.allocation_type === MachineSetSpecMachineAllocationType.Unlimited
            ? 'unlimited'
            : (allocationCfg.machine_count ?? 0),
      }
    }

    if (ms.spec.delete_strategy || ms.spec.delete_strategy_config) {
      machineSet.deleteStrategy = {
        type: ms.spec.delete_strategy ?? MachineSetSpecUpdateStrategy.Unset,
        config: ms.spec.delete_strategy_config,
      }
    }

    if (ms.spec.update_strategy || ms.spec.update_strategy_config) {
      machineSet.updateStrategy = {
        type: ms.spec.update_strategy ?? MachineSetSpecUpdateStrategy.Unset,
        config: ms.spec.update_strategy_config,
      }
    }

    const configPatch = patchesByID[formatPatchID(ms.metadata.id!, PatchBaseWeightMachineSet)]
    if (configPatch) {
      machineSet.patches[PatchID.Default] = {
        data: configPatch.spec.data!,
        weight: PatchBaseWeightMachineSet,
      }

      resources.push(configPatch)
    }

    machineSetsByID[ms.metadata.id!] = machineSet

    machineSetList.push(machineSet)

    if (!isCP) {
      state.value.index++
    }

    resources.push(ms)
  }

  // get machine set nodes
  const machineSetNodes: Resource<MachineSetNode>[] = await ResourceService.List(
    {
      type: MachineSetNodeType,
      namespace: DefaultNamespace,
    },
    withRuntime(Runtime.Omni),
    withSelectors([`${LabelCluster}=${clusterName}`]),
  )

  const machinesByID: Record<string, MachineSetNode> = {}

  for (const msn of machineSetNodes) {
    const msName = msn.metadata.labels?.[LabelMachineSet]
    if (!msName) {
      continue
    }

    const machineSetNode: {
      patches: Record<string, ConfigPatch>
      systemExtensions?: string[]
    } = {
      patches: {},
    }

    if (!machineSetsByID[msName]) {
      continue
    }

    machineSetsByID[msName].machines[msn.metadata.id!] = machineSetNode
    machinesByID[msn.metadata.id!] = machineSetNode

    const key = `cm-${msn.metadata.id!}`
    const id = formatPatchID(key, PatchBaseWeightClusterMachine)

    const configPatch = patchesByID[id]
    if (configPatch) {
      machineSetNode.patches[key] = {
        data: configPatch.spec.data!,
        weight: PatchBaseWeightMachineSet,
      }

      resources.push(configPatch)
    }

    const systemExtensions = systemExtensionsByID[`schematic-${msn.metadata.id!}`]
    if (systemExtensions) {
      machineSetNode.systemExtensions = systemExtensions.spec.extensions ?? []
    }

    if (!msn.metadata.labels?.[LabelManagedByMachineSetNodeController]) {
      resources.push(msn)
    }
  }

  state.value.machineSets = machineSetList
  state.value.baseResources = resources

  return resources
}
