// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { TalosExtensionsSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, DefaultTalosVersion, TalosExtensionsType } from '@/api/resources'

import ExtensionsPicker from './ExtensionsPicker.vue'
import { fakeExtensions } from './fakeData'

const meta: Meta<typeof ExtensionsPicker> = {
  component: ExtensionsPicker,
  args: {
    talosVersion: DefaultTalosVersion,
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data = {
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
} satisfies Story

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
