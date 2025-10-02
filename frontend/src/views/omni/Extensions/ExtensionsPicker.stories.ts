// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { TalosExtensionsSpec, TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, DefaultTalosVersion, TalosExtensionsType } from '@/api/resources'

import ExtensionsPicker from './ExtensionsPicker.vue'

faker.seed(0)
const fakeExtensions = faker.helpers.multiple(
  () => ({
    name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
    author: faker.company.name(),
    version: faker.system.semver(),
    description: faker.lorem.sentences(4),
  }),
  { count: 50 },
) satisfies TalosExtensionsSpecInfo[]

const meta: Meta<typeof ExtensionsPicker> = {
  component: ExtensionsPicker,
  args: {
    talosVersion: DefaultTalosVersion,
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  args: {
    indeterminate: true,
    modelValue: {
      [fakeExtensions[0].name]: true,
      [fakeExtensions[1].name]: true,
    },
    immutableExtensions: {
      [fakeExtensions[1].name]: true,
    },
  },
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosExtensionsSpec>({
          expectedOptions: {
            id: DefaultTalosVersion,
            type: TalosExtensionsType,
            namespace: DefaultNamespace,
          },
          initialResources: [
            {
              spec: { items: fakeExtensions },
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
