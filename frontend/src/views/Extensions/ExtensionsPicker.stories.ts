// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { DefaultTalosVersion } from '@/api/resources'

import { fakeExtensions, handlers } from './ExtensionsPicker.mocks'
import ExtensionsPicker from './ExtensionsPicker.vue'

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

  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(createWatchStreamHandler().handler)
  },
}
