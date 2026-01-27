// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'
import { RouterView } from 'vue-router'

import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { DefaultNamespace, DefaultTalosVersion, InstallationMediaConfigType } from '@/api/resources'
import * as ConfirmationStories from '@/views/omni/InstallationMedia/Steps/Confirmation.stories'

import InstallationMediaReview from './InstallationMediaReview.vue'

const meta: Meta<typeof InstallationMediaReview> = {
  component: InstallationMediaReview,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  decorators: [
    vueRouter(
      [
        {
          path: '/:presetId',
          component: RouterView,
        },
      ],
      { initialRoute: '/preset-id' },
    ),
  ],
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<InstallationMediaConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: InstallationMediaConfigType,
            id: 'preset-id',
          },
          initialResources: [
            {
              spec: {
                architecture: PlatformConfigSpecArch.ARM64,
                talos_version: DefaultTalosVersion,
                machine_labels: { 'my-label': 'my-value' },
                install_extensions: ['siderolabs/potato', 'siderolabs/tomato'],
                kernel_args: '-console console=tty0',
                secure_boot: true,
                grpc_tunnel: GrpcTunnelMode.DISABLED,
                join_token: faker.string.alphanumeric(44),
              },
              metadata: {
                namespace: DefaultNamespace,
                type: InstallationMediaConfigType,
                id: 'preset-id',
              },
            },
          ],
        }).handler,

        ...ConfirmationStories.Default.parameters.msw.handlers,
      ],
    },
  },
} satisfies Story
