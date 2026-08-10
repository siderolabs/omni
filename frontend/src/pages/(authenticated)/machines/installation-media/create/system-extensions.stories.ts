// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { AUTOMATIC_VERSION } from '@/views/InstallationMedia/useFormState'

import { handlers } from './system-extensions.mocks'
import SystemExtensions from './system-extensions.vue'

const meta: Meta<typeof SystemExtensions> = {
  component: SystemExtensions,
  args: {
    modelValue: { talosVersion: AUTOMATIC_VERSION },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
