// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'

import MaintenanceUpdate from './MaintenanceUpdate.vue'

faker.seed(0)
const talosVersions = faker.helpers.multiple<Resource<TalosVersionSpec>>(
  () => {
    const version = faker.system.semver()

    return {
      spec: { version, deprecated: faker.datatype.boolean() },
      metadata: { id: version },
    }
  },
  { count: 10 },
)

const meta: Meta<typeof MaintenanceUpdate> = {
  component: MaintenanceUpdate,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: { type: TalosVersionType, namespace: DefaultNamespace },
          initialResources: talosVersions,
        }).handler,

        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: { type: MachineStatusType, namespace: DefaultNamespace },
          initialResources: [
            {
              spec: {
                talos_version: `v${talosVersions.filter((s) => !s.spec.deprecated)[0].spec.version}`,
              },
              metadata: {},
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
