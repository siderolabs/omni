// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type { KernelArgsSpec, KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'

import UpdateKernelArgs from './UpdateKernelArgs.vue'

const meta: Meta<typeof UpdateKernelArgs> = {
  component: UpdateKernelArgs,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<KernelArgsStatusSpec>({
          expectedOptions: {
            type: KernelArgsStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: [
            {
              spec: {
                current_args: faker.helpers.multiple(() =>
                  faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                ),
                current_cmdline: faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                unmet_conditions: faker.helpers.multiple(() =>
                  faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                ),
              },
              metadata: {},
            },
          ],
        }).handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== KernelArgsType || namespace !== DefaultNamespace) return

          const resource: Resource<KernelArgsSpec> = {
            spec: {
              args: faker.helpers.multiple(() =>
                faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
              ),
            },
            metadata: {},
          }

          return HttpResponse.json({ body: JSON.stringify(resource) })
        }),
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
