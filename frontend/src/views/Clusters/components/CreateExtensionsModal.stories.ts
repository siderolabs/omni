// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import type {
  MachineStatusSpec,
  TalosExtensionsSpec,
  TalosExtensionsSpecInfo,
} from '@/api/omni/specs/omni.pb.ts'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  MachineStatusType,
  TalosExtensionsType,
} from '@/api/resources.ts'

import CreateExtensionsModal from './CreateExtensionsModal.vue'

faker.seed(0)
const machine = faker.string.uuid()

const meta: Meta<typeof CreateExtensionsModal> = {
  component: CreateExtensionsModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    machine,
    onSave: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: MachineStatusType,
            id: machine,
          },
          initialResources: [
            {
              spec: {
                schematic: { extensions: [] },
                network: { hostname: faker.internet.domainWord() },
                talos_version: `v${DefaultTalosVersion}`,
              },
              metadata: {
                namespace: DefaultNamespace,
                type: MachineStatusType,
                id: machine,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<TalosExtensionsSpec>({
          expectedOptions: {
            type: TalosExtensionsType,
            namespace: DefaultNamespace,
            id: DefaultTalosVersion,
          },
          initialResources: [
            {
              spec: {
                items: faker.helpers.multiple<TalosExtensionsSpecInfo>(
                  () => ({
                    name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
                    author: faker.company.name(),
                    version: faker.system.semver(),
                  }),
                  { count: 50 },
                ),
              },
              metadata: {},
            },
          ],
        }).handler,
      ],
    },
  },
}
