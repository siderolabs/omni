// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import {
  LabelCluster,
  LabelControlPlaneRole,
  LabelWorkerRole,
  MachineStatusLabelArch,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelCores,
  MachineStatusLabelCPU,
  MachineStatusLabelDisconnected,
  MachineStatusLabelInstance,
  MachineStatusLabelInvalidState,
  MachineStatusLabelMem,
  MachineStatusLabelNet,
  MachineStatusLabelPlatform,
  MachineStatusLabelRegion,
  MachineStatusLabelStorage,
  MachineStatusLabelTalosVersion,
  MachineStatusLabelZone,
} from '@/api/resources'

import ItemLabels from './ItemLabels.vue'

const meta: Meta<typeof ItemLabels> = {
  component: ItemLabels,
  args: {
    addLabelFunc: fn(),
    removeLabelFunc: fn(),
    resource: {
      spec: {},
      metadata: {
        id: faker.string.uuid(),
        labels: faker.helpers
          .multiple(
            () => [
              faker.helpers.arrayElement([
                LabelCluster,
                MachineStatusLabelAvailable,
                MachineStatusLabelInvalidState,
                MachineStatusLabelConnected,
                MachineStatusLabelDisconnected,
                MachineStatusLabelPlatform,
                MachineStatusLabelCores,
                MachineStatusLabelMem,
                MachineStatusLabelStorage,
                MachineStatusLabelNet,
                MachineStatusLabelCPU,
                MachineStatusLabelArch,
                MachineStatusLabelRegion,
                MachineStatusLabelZone,
                MachineStatusLabelInstance,
                MachineStatusLabelTalosVersion,
                LabelControlPlaneRole,
                LabelWorkerRole,
              ]),
              faker.helpers.maybe(() =>
                faker.helpers.arrayElement([faker.hacker.verb(), faker.lorem.sentence()]),
              ),
            ],
            {
              count: 20,
            },
          )
          .reduce<Record<string, string>>(
            (prev, [key, value = '']) => ({ ...prev, [key]: value }),
            {},
          ),
      },
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
