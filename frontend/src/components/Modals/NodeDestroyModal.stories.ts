// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec, MachineSetNodeSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelControlPlaneRole,
  LabelMachineSet,
  MachineSetNodeType,
} from '@/api/resources'
import { controlPlaneMachineSetId } from '@/methods/machineset'

import NodeDestroyModal from './NodeDestroyModal.vue'

const clusterId = faker.word.noun()
const machineId = faker.string.uuid()
const nodeName = `machine-${machineId}`

const meta: Meta<typeof NodeDestroyModal> = {
  component: NodeDestroyModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    onOnDestroy: fn(),
    clusterId,
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

function makeClusterMachineStatus(isControlPlane: boolean): Resource<ClusterMachineStatusSpec> {
  const labels: Record<string, string> = {
    [ClusterMachineStatusLabelNodeName]: nodeName,
  }

  if (isControlPlane) {
    labels[LabelControlPlaneRole] = ''
  }

  return {
    spec: {},
    metadata: {
      namespace: DefaultNamespace,
      type: ClusterMachineStatusType,
      id: machineId,
      labels,
    },
  }
}

function clusterMachineStatusHandler(isControlPlane: boolean) {
  return createWatchStreamHandler<ClusterMachineStatusSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: ClusterMachineStatusType,
      id: machineId,
    },
    initialResources: [makeClusterMachineStatus(isControlPlane)],
  }).handler
}

function machineSetNodesHandler(count: number) {
  return createWatchStreamHandler<MachineSetNodeSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: MachineSetNodeType,
    },
    initialResources: faker.helpers.multiple(
      () => ({
        spec: {},
        metadata: {
          namespace: DefaultNamespace,
          type: MachineSetNodeType,
          id: faker.string.uuid(),
          labels: {
            [LabelMachineSet]: controlPlaneMachineSetId(clusterId),
            [LabelControlPlaneRole]: '',
          },
        },
      }),
      { count },
    ),
  }).handler
}

export const Worker: Story = {
  parameters: {
    msw: {
      handlers: [clusterMachineStatusHandler(false), machineSetNodesHandler(3)],
    },
  },
}

export const ControlPlane: Story = {
  parameters: {
    msw: {
      handlers: [clusterMachineStatusHandler(true), machineSetNodesHandler(2)],
    },
  },
}

export const ControlPlaneQuorumWarning: Story = {
  parameters: {
    msw: {
      handlers: [clusterMachineStatusHandler(true), machineSetNodesHandler(3)],
    },
  },
}
