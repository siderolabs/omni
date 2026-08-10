// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { AUTOMATIC_VERSION } from '@/views/InstallationMedia/useFormState'

import { handlers, SBC } from './extra-args.mocks'
import ExtraArgs from './extra-args.vue'

const meta: Meta<typeof ExtraArgs> = {
  component: ExtraArgs,
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

export const Pre1_10: Story = {
  ...Default,
  name: 'Pre-1.10',
  args: {
    modelValue: {
      secureBoot: false,
      talosVersion: '1.9.0',
    },
  },
}

export const WithSecureBoot: Story = {
  ...Default,
  args: {
    modelValue: {
      secureBoot: true,
    },
  },
}

export const WithOverlayOptions: Story = {
  ...Default,
  args: {
    modelValue: {
      talosVersion: AUTOMATIC_VERSION,
      hardwareType: 'sbc',
      sbcType: SBC.metadata.id,
    },
  },
}
