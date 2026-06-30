// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Empty } from '@/api/google/protobuf/empty.pb.ts'
import type {
  DestroyServiceAccountRequest,
  DestroyUserRequest,
} from '@/api/omni/management/management.pb.ts'

import UserDestroyModal from './UserDestroyModal.vue'

const meta: Meta<typeof UserDestroyModal> = {
  component: UserDestroyModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
  },
  parameters: {
    msw: {
      handlers: [
        http.post<never, DestroyServiceAccountRequest, Empty>(
          '/management.ManagementService/DestroyServiceAccount',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),

        http.post<never, DestroyUserRequest, Empty>(
          '/management.ManagementService/DestroyUser',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const DeleteUser: Story = {
  args: {
    identity: faker.internet.email(),
  },
}

export const DeleteServiceAccount: Story = {
  args: {
    identity: 'talemu@infra-provider.serviceaccount.omni.sidero.dev',
    isServiceAccount: true,
  },
}
