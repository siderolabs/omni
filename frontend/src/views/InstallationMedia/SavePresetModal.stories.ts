// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'

import { handlers } from './SavePresetModal.mocks'
import SavePresetModal from './SavePresetModal.vue'

const meta: Meta<typeof SavePresetModal> = {
  component: SavePresetModal,
  args: {
    open: true,
    formState: {
      hardwareType: 'metal',
      talosVersion: '1.11.5',
      joinToken: 'w7uVuW3zbVKIYQuzEcyetAHeYMeo5q2L9RvkAVfCfSCD',
      machineArch: PlatformConfigSpecArch.AMD64,
      systemExtensions: ['siderolabs/crun', 'siderolabs/chelsio-drivers'],
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
