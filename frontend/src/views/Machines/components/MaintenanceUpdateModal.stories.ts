// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { compare, minor } from 'semver'

import type { Resource } from '@/api/grpc'
import type {
  MaintenanceUpgradeRequest,
  MaintenanceUpgradeResponse,
} from '@/api/omni/management/management.pb.ts'
import { type MachineStatusSpec, type TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  MachineStatusSnapshotType,
  MachineStatusType,
  TalosVersionType,
} from '@/api/resources'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb.ts'

import MaintenanceUpdate from './MaintenanceUpdateModal.vue'

faker.seed(0)

const machineId = faker.string.uuid()

const versions = faker.helpers
  .uniqueArray(
    () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .sort(compare)

const talosVersions = versions.map<Resource<TalosVersionSpec>>((version) => ({
  spec: {
    version,
    deprecated: faker.datatype.boolean(),
    unsupported: faker.datatype.boolean(),
    upgradable_talos_versions: versions.filter((v) => minor(v) === minor(version) + 1),
  },
  metadata: {
    id: version,
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
}))

const meta: Meta<typeof MaintenanceUpdate> = {
  component: MaintenanceUpdate,
  args: {
    open: true,
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: talosVersions,
        }).handler,

        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            type: MachineStatusType,
            namespace: DefaultNamespace,
            id: machineId,
          },
          initialResources: [
            {
              spec: {
                talos_version: `v${talosVersions.filter((s) => !s.spec.deprecated).at(-(talosVersions.length / 4))?.spec.version}`,
              },
              metadata: {
                type: MachineStatusType,
                namespace: DefaultNamespace,
                id: machineId,
              },
            },
          ],
        }).handler,

        http.post<never, MaintenanceUpgradeRequest, MaintenanceUpgradeResponse>(
          '/management.ManagementService/MaintenanceUpgrade',
          async () => {
            await delay(1_000)

            return HttpResponse.json({} as MaintenanceUpgradeResponse)
          },
        ),
      ],
    },
  },
} satisfies Story

export const AlreadyInProgress: Story = {
  args: {
    snapshot: {
      spec: {
        machine_status: {
          stage: MachineStatusEventMachineStage.INSTALLING,
        },
      },
      metadata: {
        type: MachineStatusSnapshotType,
        namespace: DefaultNamespace,
        id: machineId,
      },
    },
  },
  parameters: {
    msw: {
      handlers: Data.parameters.msw.handlers,
    },
  },
}
