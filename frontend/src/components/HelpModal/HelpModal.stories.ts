// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { SupportSpec } from '@/api/omni/specs/virtual.pb'
import { SupportID, SupportType, VirtualNamespace } from '@/api/resources'

import HelpModal from './HelpModal.vue'

const meta: Meta<typeof HelpModal> = {
  component: HelpModal,
  args: {
    open: true,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [getSupportEnabledHandler(true)],
    },
  },
}

export const NoSupport: Story = {
  parameters: {
    msw: {
      handlers: [getSupportEnabledHandler(false)],
    },
  },
}

function getSupportEnabledHandler(enabled: boolean) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { id, type, namespace } = await request.clone().json()

      if (id !== SupportID || type !== SupportType || namespace !== VirtualNamespace) {
        return
      }

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: {
            namespace: VirtualNamespace,
            type: SupportType,
            id: SupportID,
          },
          spec: {
            support_enabled: enabled,
          },
        } as Resource<SupportSpec>),
      })
    },
  )
}
