// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { compare, minor } from 'semver'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'

import MaintenanceUpdate from './MaintenanceUpdateModal.vue'

faker.seed(0)

const versions = faker.helpers.uniqueArray(
  () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
  40,
)

const talosVersions = versions.sort(compare).map<Resource<TalosVersionSpec>>((version) => ({
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
    machineId: faker.string.uuid(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
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
          },
          initialResources: [
            {
              spec: {
                talos_version: `v${talosVersions.filter((s) => !s.spec.deprecated).at(-(talosVersions.length / 4))?.spec.version}`,
              },
              metadata: {
                type: MachineStatusType,
                namespace: DefaultNamespace,
              },
            },
          ],
        }).handler,
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
