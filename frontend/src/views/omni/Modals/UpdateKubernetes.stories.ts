// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import * as semver from 'semver'

import type { Resource } from '@/api/grpc'
import type { KubernetesUpgradeStatusSpec, KubernetesVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  KubernetesUpgradeStatusType,
  KubernetesVersionType,
} from '@/api/resources'

import UpdateKubernetes from './UpdateKubernetes.vue'

const versions = faker.helpers
  .uniqueArray(
    () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .sort(semver.compare)

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
            const upgrade_versions = versions.filter((v) =>
              [30, 31].includes(semver.parse(v)!.minor),
            )

            const [last_upgrade_version] = upgrade_versions
              .filter((v) => semver.parse(v)?.minor === 30)
              .splice(-3, 1)

            return [
              {
                spec: { last_upgrade_version, upgrade_versions },
                metadata: {},
              },
            ]
          },
        }).handler,

        createWatchStreamHandler<KubernetesVersionSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: KubernetesVersionType,
          },
          initialResources: versions.map<Resource<KubernetesVersionSpec>>((version) => ({
            spec: { version },
            metadata: { id: version },
          })),
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
