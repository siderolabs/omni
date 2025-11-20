// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { currentUser } from '@/methods/auth'

import UserConsent from './UserConsent.vue'

/**
 * Internal UserConsent logic blocks rendering if no currentUser
 * is set.
 *
 * This probably would be better server in a parent component,
 * and the currentUser probably should be part of a provide/inject
 * flow for testability and stories.
 */
currentUser.value = {
  spec: {},
  metadata: {},
}

const meta: Meta<typeof UserConsent> = {
  component: UserConsent,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace, id } = await request.clone().json()

          if (
            type !== FeaturesConfigType ||
            namespace !== DefaultNamespace ||
            id !== FeaturesConfigID
          )
            return

          const resource: Resource<FeaturesConfigSpec> = {
            spec: {
              user_pilot_settings: {
                app_token: 'token',
              },
            },
            metadata: {},
          }

          return HttpResponse.json({ body: JSON.stringify(resource) })
        }),
      ],
    },
  },
}
