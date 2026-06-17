// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Resource } from '@/api/grpc'
import type {
  MachinePowerOffRequest,
  MachinePowerOffResponse,
} from '@/api/omni/management/management.pb.ts'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  ClusterPermissionsType,
  DefaultNamespace,
  VirtualNamespace,
} from '@/api/resources'

import NodeShutdownModal from './NodeShutdownModal.vue'

const clusterId = faker.word.noun()
const machineId = faker.string.uuid()
const nodeName = `machine-${machineId}`

const meta: Meta<typeof NodeShutdownModal> = {
  component: NodeShutdownModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    clusterId,
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

function clusterMachineStatusHandler(name: string) {
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

function clusterPermissionsHandler(canReboot: boolean) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { id, type, namespace } = await request.clone().json()

      if (id !== clusterId || type !== ClusterPermissionsType || namespace !== VirtualNamespace) {
        return
      }

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: {
            namespace: VirtualNamespace,
            type: ClusterPermissionsType,
            id: clusterId,
          },
          spec: {
            can_reboot_machines: canReboot,
          },
        } as Resource<ClusterPermissionsSpec>),
      })
    },
  )
}

function powerOffHandler() {
  return http.post<never, MachinePowerOffRequest, MachinePowerOffResponse>(
    '/management.ManagementService/MachinePowerOff',
    async () => {
      await delay()

      return HttpResponse.json({})
    },
  )
}

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        clusterMachineStatusHandler(nodeName),
        clusterPermissionsHandler(true),
        powerOffHandler(),
      ],
    },
  },
}
