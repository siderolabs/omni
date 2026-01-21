// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'

import type { CreateRequest, CreateResponse } from '@/api/omni/resources/resources.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'

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

export const Default = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<InstallationMediaConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: InstallationMediaConfigType,
          },
          initialResources: (a) => {
            if (a.id !== 'existing') return []

            return [
              {
                spec: {},
                metadata: {
                  namespace: DefaultNamespace,
                  type: InstallationMediaConfigType,
                  id: 'existing',
                },
              },
            ]
          },
        }).handler,

        http.post<never, CreateRequest, CreateResponse>(
          '/omni.resources.ResourceService/Create',
          async ({ request }) => {
            const { resource } = await request.clone().json()

            if (!resource?.metadata) return

            const { type, namespace } = resource.metadata

            if (type !== InstallationMediaConfigType || namespace !== DefaultNamespace) return

            await delay(1_000)

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
} satisfies Story
