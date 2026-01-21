// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import * as semver from 'semver'

import type { TalosUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosUpgradeStatusType } from '@/api/resources'

import UpdateTalos from './UpdateTalos.vue'

const upgrade_versions = faker.helpers
  .uniqueArray(
    () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .sort(semver.compare)

const meta: Meta<typeof UpdateTalos> = {
  component: UpdateTalos,
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
            const [last_upgrade_version] = upgrade_versions.splice(
              Math.round(upgrade_versions.length / 2),
              1,
            )

            return [
              {
                spec: { last_upgrade_version, upgrade_versions },
                metadata: {},
              },
            ]
          },
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
