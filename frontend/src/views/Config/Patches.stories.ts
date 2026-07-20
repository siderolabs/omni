// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { ClusterMachineStatusSpec, ConfigPatchSpec } from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec, PermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterMachineStatusType,
  ClusterPermissionsType,
  ConfigPatchDescription,
  ConfigPatchName,
  ConfigPatchType,
  ControlPlanesIDSuffix,
  DefaultNamespace,
  DefaultWorkersIDSuffix,
  LabelCluster,
  LabelClusterMachine,
  LabelDisabled,
  LabelHostname,
  LabelMachine,
  LabelMachineSet,
  PermissionsID,
  PermissionsType,
  VirtualNamespace,
} from '@/api/resources'

import Patches from './Patches.vue'

const CLUSTER = 'talos-default'
const MACHINE = 'e3c0d1a2-0000-0000-0000-000000000001'

// Two cluster machines whose hostnames the page resolves from ClusterMachineStatus.
const clusterMachines = [
  { id: MACHINE, hostname: 'talos-cp-1' },
  { id: 'a1b2c3d4-0000-0000-0000-000000000002', hostname: 'talos-worker-1' },
]

function machineStatus(id: string, hostname: string): Resource<ClusterMachineStatusSpec> {
  return {
    metadata: {
      namespace: DefaultNamespace,
      type: ClusterMachineStatusType,
      id,
      labels: { [LabelHostname]: hostname, [LabelCluster]: CLUSTER },
    },
    spec: {},
  }
}

interface PatchInit {
  id: string
  name?: string
  description?: string
  disabled?: boolean
  labels: Record<string, string>
}

function makePatch({
  id,
  name,
  description,
  disabled,
  labels,
}: PatchInit): Resource<ConfigPatchSpec> {
  const annotations: Record<string, string> = {}
  if (name) annotations[ConfigPatchName] = name
  if (description) annotations[ConfigPatchDescription] = description

  return {
    metadata: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id,
      labels: disabled ? { ...labels, [LabelDisabled]: '' } : labels,
      annotations: Object.keys(annotations).length ? annotations : undefined,
    },
    spec: {
      data: 'machine:\n  network:\n    hostname: example\n',
    },
  }
}

// A set of patches that exercises every grouping the page renders: cluster-level,
// control planes, default workers, additional workers and a specific cluster machine.
const clusterPatches: Resource<ConfigPatchSpec>[] = [
  makePatch({
    id: '400-cluster-wide',
    name: 'cluster-defaults',
    description: 'Baseline configuration applied to every machine in the cluster',
    labels: { [LabelCluster]: CLUSTER },
  }),
  makePatch({
    id: '500-control-plane-tuning',
    name: 'control-plane-tuning',
    description: 'API server flags for the control plane',
    labels: { [LabelCluster]: CLUSTER, [LabelMachineSet]: `${CLUSTER}-${ControlPlanesIDSuffix}` },
  }),
  makePatch({
    id: '500-worker-kubelet',
    name: 'worker-kubelet',
    description: 'Kubelet extra args for the default worker set',
    labels: { [LabelCluster]: CLUSTER, [LabelMachineSet]: `${CLUSTER}-${DefaultWorkersIDSuffix}` },
  }),
  makePatch({
    id: '500-gpu-drivers',
    name: 'gpu-drivers',
    description: 'System extensions for the GPU worker pool',
    disabled: true,
    labels: { [LabelCluster]: CLUSTER, [LabelMachineSet]: `${CLUSTER}-gpu` },
  }),
  makePatch({
    id: '500-cp-1-hostname',
    name: 'cp-1-hostname',
    description: 'Per-machine override for the first control plane node',
    labels: { [LabelCluster]: CLUSTER, [LabelClusterMachine]: clusterMachines[0].id },
  }),
  makePatch({
    id: '999-no-metadata',
    labels: { [LabelCluster]: CLUSTER },
  }),
]

function clusterPatchesHandler(patches = clusterPatches) {
  return createWatchStreamHandler<ConfigPatchSpec>({
    expectedOptions: { namespace: DefaultNamespace, type: ConfigPatchType },
    initialResources: patches,
  }).handler
}

const machineStatusesHandler = createWatchStreamHandler<ClusterMachineStatusSpec>({
  expectedOptions: { namespace: DefaultNamespace, type: ClusterMachineStatusType },
  initialResources: clusterMachines.map((m) => machineStatus(m.id, m.hostname)),
}).handler

// Answers the ClusterPermissions Get for the cluster-scoped page.
function clusterPermissionsHandler(canManage: boolean) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { type, namespace } = await request.clone().json()

      if (type !== ClusterPermissionsType || namespace !== VirtualNamespace) return

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: { namespace: VirtualNamespace, type: ClusterPermissionsType, id: CLUSTER },
          spec: {
            can_read_config_patches: true,
            can_manage_config_patches: canManage,
          },
        } satisfies Resource<ClusterPermissionsSpec>),
      })
    },
  )
}

// Answers the Permissions Get for the machine-scoped page.
function machinePermissionsHandler(canManage: boolean) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { type, namespace } = await request.clone().json()

      if (type !== PermissionsType || namespace !== VirtualNamespace) return

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: { namespace: VirtualNamespace, type: PermissionsType, id: PermissionsID },
          spec: {
            can_read_machine_config_patches: true,
            can_manage_machine_config_patches: canManage,
          },
        } satisfies Resource<PermissionsSpec>),
      })
    },
  )
}

const meta: Meta<typeof Patches> = {
  component: Patches,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    () => ({
      template: '<div class="h-screen bg-naturals-n0 p-8"><story /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

/** Cluster-scoped view with patches spread across every group and full manage permissions. */
export const Cluster: Story = {
  args: { cluster: CLUSTER },
  parameters: {
    msw: {
      handlers: [clusterPatchesHandler(), machineStatusesHandler, clusterPermissionsHandler(true)],
    },
  },
}

/** Same data, but the user cannot manage patches — create, toggle and delete are disabled. */
export const ClusterReadOnly: Story = {
  args: { cluster: CLUSTER },
  parameters: {
    msw: {
      handlers: [clusterPatchesHandler(), machineStatusesHandler, clusterPermissionsHandler(false)],
    },
  },
}

/** Machine-scoped view: patches attached to a single machine. */
export const Machine: Story = {
  args: { machine: MACHINE },
  parameters: {
    msw: {
      handlers: [
        clusterPatchesHandler([
          makePatch({
            id: '500-machine-network',
            name: 'machine-network',
            description: 'Static network configuration for this machine',
            labels: { [LabelMachine]: MACHINE },
          }),
          makePatch({
            id: '500-machine-disabled',
            name: 'experimental',
            description: 'A disabled experimental patch',
            disabled: true,
            labels: { [LabelMachine]: MACHINE },
          }),
        ]),
        machineStatusesHandler,
        machinePermissionsHandler(true),
      ],
    },
  },
}

/** No patches associated — renders the empty-state alert. */
export const Empty: Story = {
  args: { cluster: CLUSTER },
  parameters: {
    msw: {
      handlers: [
        clusterPatchesHandler([]),
        machineStatusesHandler,
        clusterPermissionsHandler(true),
      ],
    },
  },
}

/** Watches never bootstrap — the loading spinner stays visible. */
export const Loading: Story = {
  args: { cluster: CLUSTER },
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ConfigPatchSpec>({
          skipBootstrap: true,
          expectedOptions: { namespace: DefaultNamespace, type: ConfigPatchType },
        }).handler,
        createWatchStreamHandler<ClusterMachineStatusSpec>({
          skipBootstrap: true,
          expectedOptions: { namespace: DefaultNamespace, type: ClusterMachineStatusType },
        }).handler,
        clusterPermissionsHandler(true),
      ],
    },
  },
}
