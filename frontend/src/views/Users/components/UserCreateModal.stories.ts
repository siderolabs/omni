// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { CreateUserRequest, CreateUserResponse } from '@/api/omni/management/management.pb.ts'

import UserCreateModal from './UserCreateModal.vue'

const meta: Meta<typeof UserCreateModal> = {
  component: UserCreateModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, CreateUserRequest, CreateUserResponse>(
          '/management.ManagementService/CreateUser',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
