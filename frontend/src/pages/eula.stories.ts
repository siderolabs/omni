// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'

import type { CreateRequest, CreateResponse } from '@/api/omni/resources/resources.pb'
import { DefaultNamespace, EulaAcceptanceID, EulaAcceptanceType } from '@/api/resources'

import Eula from './eula.vue'

const meta: Meta<typeof Eula> = {
  component: Eula,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, CreateRequest, CreateResponse>(
          '/omni.resources.ResourceService/Create',
          async ({ request }) => {
            const { resource } = await request.clone().json()

            if (!resource?.metadata) return

            const { id, type, namespace } = resource.metadata

            if (
              id !== EulaAcceptanceID ||
              type !== EulaAcceptanceType ||
              namespace !== DefaultNamespace
            ) {
              return
            }

            await delay(1_000)

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
