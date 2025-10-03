// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { KubernetesUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KubernetesUpgradeStatusType } from '@/api/resources'

import UpdateKubernetes from './UpdateKubernetes.vue'

const meta: Meta<typeof UpdateKubernetes> = {
  component: UpdateKubernetes,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<KubernetesUpgradeStatusSpec>({
          expectedOptions: {
            type: KubernetesUpgradeStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: () => {
            faker.seed(0)

            const upgrade_versions = faker.helpers
              .multiple(() => faker.system.semver(), { count: 10 })
              .sort()

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
