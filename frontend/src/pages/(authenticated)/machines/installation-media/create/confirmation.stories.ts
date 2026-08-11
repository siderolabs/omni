// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { AUTOMATIC_VERSION } from '@/views/InstallationMedia/useFormState'

import { handlers } from './confirmation.mocks'
import Confirmation from './confirmation.vue'

const meta: Meta<typeof Confirmation> = {
  component: Confirmation,
  args: {
    modelValue: {
      hardwareType: 'metal',
      machineArch: PlatformConfigSpecArch.ARM64,
      talosVersion: AUTOMATIC_VERSION,
      machineUserLabels: {
        'my-label': { canRemove: true, value: 'my-value' },
      },
      systemExtensions: ['siderolabs/potato', 'siderolabs/tomato'],
      cmdline: '-console console=tty0',
      secureBoot: false,
      useGrpcTunnel: false,
      joinToken: faker.string.alphanumeric(44),
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Enterprise: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers())
  },
}

export const EnterpriseSecureBoot: Story = {
  args: {
    modelValue: {
      ...meta.args?.modelValue,
      secureBoot: true,
    },
  },
  beforeEach({ msw }) {
    msw.use(...handlers())
  },
}

export const NotEnterprise: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers(false))
  },
}
