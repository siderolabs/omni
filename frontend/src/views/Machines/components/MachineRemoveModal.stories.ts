// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc.ts'
import type {
  DeleteRequest,
  DeleteResponse,
  ListRequest,
  ListResponse,
} from '@/api/omni/resources/resources.pb.ts'
import type { ConfigPatchSpec } from '@/api/omni/specs/omni.pb.ts'
import { ConfigPatchType, DefaultNamespace } from '@/api/resources.ts'

import MachineRemoveModal from './MachineRemoveModal.vue'

const meta: Meta<typeof MachineRemoveModal> = {
  component: MachineRemoveModal,
  args: {
    open: true,
    machines: faker.helpers.multiple(faker.hacker.noun),
    clusters: [],
  },
  parameters: {
    msw: {
      handlers: [
        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Teardown',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),

        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Delete',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),

        http.post<never, ListRequest, ListResponse>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== ConfigPatchType || namespace !== DefaultNamespace) return

            const patches: Resource<ConfigPatchSpec>[] = [
              {
                spec: {},
                metadata: {
                  id: faker.string.uuid(),
                  namespace,
                  type,
                },
              },
            ]

            await delay()

            return HttpResponse.json({
              items: patches.map((p) => JSON.stringify(p)),
              total: patches.length,
            })
          },
        ),
      ],
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const WithClusters: Story = {
  args: {
    clusters: faker.helpers.multiple(faker.string.uuid),
  },
}
