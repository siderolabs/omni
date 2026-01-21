// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { formatRFC3339 } from 'date-fns'

import {
  KubernetesUpgradeStatusSpecPhase,
  type OngoingTaskSpec,
  TalosUpgradeStatusSpecPhase,
} from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'

import OngoingTasks from './OngoingTasks.vue'

const meta: Meta<typeof OngoingTasks> = {
  component: OngoingTasks,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<OngoingTaskSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: OngoingTaskType,
          },
          initialResources: faker.helpers.multiple(
            () => ({
              metadata: {
                id: faker.string.uuid(),
                created: formatRFC3339(faker.date.recent()),
              },
              spec: {
                title: faker.string.uuid(),
                ...faker.helpers.arrayElement([
                  {
                    kubernetes_upgrade: {
                      last_upgrade_version: faker.system.semver(),
                      current_upgrade_version: faker.system.semver(),
                      phase: KubernetesUpgradeStatusSpecPhase.Upgrading,
                    },
                  },
                  {
                    talos_upgrade: {
                      last_upgrade_version: faker.system.semver(),
                      current_upgrade_version: faker.system.semver(),
                      phase: TalosUpgradeStatusSpecPhase.Upgrading,
                    },
                  },
                  {
                    talos_upgrade: {
                      last_upgrade_version: faker.system.semver(),
                      phase: TalosUpgradeStatusSpecPhase.Reverting,
                    },
                  },
                  {
                    machine_upgrade: {
                      schematic_id: faker.string.hexadecimal({
                        prefix: '',
                        casing: 'lower',
                        length: 64,
                      }),
                      current_schematic_id: faker.string.hexadecimal({
                        prefix: '',
                        casing: 'lower',
                        length: 64,
                      }),
                    },
                  },
                  {
                    destroy: {
                      phase: 'Destroying: 2 machine sets, 3 machines',
                    },
                  },
                ]),
              },
            }),
            { count: 10 },
          ),
        }).handler,
      ],
    },
  },
}
