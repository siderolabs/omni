// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { compare, minor } from 'semver'

import type { Resource } from '@/api/grpc.ts'
import type { TalosUpgradeStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosUpgradeStatusType, TalosVersionType } from '@/api/resources'

import UpdateTalos from './UpdateTalosModal.vue'

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

const meta: Meta<typeof UpdateTalos> = {
  component: UpdateTalos,
  args: {
    open: true,
    clusterName: 'talos-default',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosUpgradeStatusSpec>({
          expectedOptions: {
            type: TalosUpgradeStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: () => {
            const [last_upgrade_version] = versions.splice(Math.round(versions.length / 2), 1)

            return [
              {
                spec: {
                  last_upgrade_version,
                  upgrade_versions: versions,
                },
                metadata: {
                  id: faker.string.uuid(),
                  type: TalosUpgradeStatusType,
                  namespace: DefaultNamespace,
                },
              },
            ]
          },
        }).handler,

        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: talosVersions,
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
