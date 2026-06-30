// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Empty } from '@/api/google/protobuf/empty.pb.ts'
import type { Resource } from '@/api/grpc.ts'
import type { UpdateUserRequest } from '@/api/omni/management/management.pb.ts'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb.ts'
import type { UserSpec } from '@/api/omni/specs/auth.pb'
import { DefaultNamespace, RoleAdmin, UserType } from '@/api/resources.ts'

import RoleEditModal from './RoleEditModal.vue'

const identity = faker.internet.email()
const userId = faker.string.uuid()

const meta: Meta<typeof RoleEditModal> = {
  component: RoleEditModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
  },
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
            const { id, type, namespace } = await request.clone().json()

            if (type !== UserType || namespace !== DefaultNamespace) {
              return
            }

            await delay()

            return HttpResponse.json({
              body: JSON.stringify({
                metadata: { namespace, type, id },
                spec: { role: RoleAdmin },
              } as Resource<UserSpec>),
            })
          },
        ),

        http.post<never, UpdateUserRequest, Empty>(
          '/management.ManagementService/UpdateUser',
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

export const EditUser: Story = {
  args: {
    identity,
    userId,
  },
}

export const EditServiceAccount: Story = {
  args: {
    identity,
    userId,
    isServiceAccount: true,
  },
}
