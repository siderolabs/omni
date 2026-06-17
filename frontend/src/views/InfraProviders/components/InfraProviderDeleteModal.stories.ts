// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { DeleteRequest, DeleteResponse } from '@/api/omni/resources/resources.pb.ts'

import InfraProviderDeleteModal from './InfraProviderDeleteModal.vue'

const meta: Meta<typeof InfraProviderDeleteModal> = {
  component: InfraProviderDeleteModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    providerId: 'my pretty provider',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Delete',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
