// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import {
  type ExtensionsConfigurationSpec,
  type MachineExtensionsStatusSpec,
  MachineExtensionsStatusSpecItemPhase,
} from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  ExtensionsConfigurationType,
  LabelCluster,
  LabelClusterMachine,
  MachineExtensionsStatusType,
} from '@/api/resources'
import * as ExtensionsPickerStories from '@/views/Extensions/ExtensionsPicker.stories'
import { fakeExtensions } from '@/views/Extensions/fakeData'

import UpdateExtensionsModal from './UpdateExtensionsModal.vue'

const machineId = 'machine'
const clusterId = 'cluster'

const meta: Meta<typeof UpdateExtensionsModal> = {
  component: UpdateExtensionsModal,
  args: {
    open: true,
    clusterId,
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

const extensions = faker.helpers.arrayElements(fakeExtensions, 20).map((e) => ({
  name: e.name,
  immutable: faker.datatype.boolean(),
  phase: faker.helpers.enumValue(MachineExtensionsStatusSpecItemPhase),
}))

const baseHandlers = [
  ...ExtensionsPickerStories.Data.parameters.msw.handlers,

  createWatchStreamHandler<MachineExtensionsStatusSpec>({
    expectedOptions: {
      id: machineId,
      namespace: DefaultNamespace,
      type: MachineExtensionsStatusType,
    },
    initialResources: [
      {
        metadata: {
          id: machineId,
          namespace: DefaultNamespace,
          type: MachineExtensionsStatusType,
        },
        spec: {
          extensions,
          talos_version: `v${DefaultTalosVersion}`,
        },
      },
    ],
  }).handler,
]

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        ...baseHandlers,

        createWatchStreamHandler<ExtensionsConfigurationSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ExtensionsConfigurationType,
            selectors: {
              [LabelCluster]: clusterId,
            },
          },
          initialResources: [
            {
              metadata: {
                namespace: DefaultNamespace,
                type: ExtensionsConfigurationType,
                id: faker.string.uuid(),
                labels: {
                  [LabelCluster]: clusterId,
                  [LabelClusterMachine]: faker.string.uuid(),
                },
              },
              spec: {
                extensions: extensions.map((e) => e.name),
              },
            },
          ],
        }).handler,
      ],
    },
  },
}

export const Indeterminate: Story = {
  parameters: {
    msw: {
      handlers: [
        ...baseHandlers,

        createWatchStreamHandler<ExtensionsConfigurationSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ExtensionsConfigurationType,
            selectors: {
              [LabelCluster]: clusterId,
            },
          },
          initialResources: [
            {
              metadata: {
                namespace: DefaultNamespace,
                type: ExtensionsConfigurationType,
                id: faker.string.uuid(),
                labels: {
                  [LabelCluster]: clusterId,
                },
              },
              spec: {
                extensions: extensions.map((e) => e.name),
              },
            },
          ],
        }).handler,
      ],
    },
  },
}
