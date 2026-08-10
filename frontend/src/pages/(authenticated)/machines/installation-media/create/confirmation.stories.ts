// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { type FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
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
      secureBoot: true,
      useGrpcTunnel: false,
      joinToken: faker.string.alphanumeric(44),
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

export const NotEnterprise: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<FeaturesConfigSpec>({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: FeaturesConfigType,
          id: FeaturesConfigID,
        },
        initialResources: [
          {
            spec: {
              image_factory_base_url: 'https://factory.talos.dev',
              image_factory_pxe_base_url: 'https://pxe.factory.talos.dev',
              is_enterprise_image_factory: false,
            },
            metadata: {
              namespace: DefaultNamespace,
              type: FeaturesConfigType,
              id: FeaturesConfigID,
            },
          },
        ],
      }).handler,
      ...handlers,
    )
  },
}
