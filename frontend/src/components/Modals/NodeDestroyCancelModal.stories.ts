// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import type { ClusterMachineSpec, ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  ClusterMachineType,
  DefaultNamespace,
} from '@/api/resources'

import NodeDestroyCancelModal from './NodeDestroyCancelModal.vue'

const machineId = faker.string.uuid()
const name = `machine-${machineId}`

const meta: Meta<typeof NodeDestroyCancelModal> = {
  component: NodeDestroyCancelModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

function clusterMachineHandler(phase: string) {
  return createWatchStreamHandler<ClusterMachineSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: ClusterMachineType,
      id: machineId,
    },
    initialResources: [
      {
        spec: {},
        metadata: {
          namespace: DefaultNamespace,
          type: ClusterMachineType,
          id: machineId,
          phase,
        },
      },
    ],
  }).handler
}

function clusterMachineStatusHandler() {
  return createWatchStreamHandler<ClusterMachineStatusSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: ClusterMachineStatusType,
      id: machineId,
    },
    initialResources: [
      {
        spec: {},
        metadata: {
          namespace: DefaultNamespace,
          type: ClusterMachineStatusType,
          id: machineId,
          labels: {
            [ClusterMachineStatusLabelNodeName]: name,
          },
        },
      },
    ],
  }).handler
}

export const Running: Story = {
  parameters: {
    msw: {
      handlers: [clusterMachineHandler('Running'), clusterMachineStatusHandler()],
    },
  },
}

export const NotRunning: Story = {
  parameters: {
    msw: {
      handlers: [clusterMachineHandler('Destroying'), clusterMachineStatusHandler()],
    },
  },
}
