// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createBootstrapEvent, createCreatedEvent, encodeResponse } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { WatchRequest } from '@/api/omni/resources/resources.pb'
import { type InfraMachineSpec } from '@/api/omni/specs/infra.pb'
import { InfraMachineType, InfraProviderNamespace, LabelInfraProviderID } from '@/api/resources'

import MachinesPending from './MachinesPending.vue'

const meta: Meta<typeof MachinesPending> = {
  component: MachinesPending,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, WatchRequest>(
          '/omni.resources.ResourceService/Watch',
          async ({ request }) => {
            const { type, namespace, limit = 10, offset = 0 } = await request.json()

            if (type !== InfraMachineType || namespace !== InfraProviderNamespace) return

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                faker.seed(offset)

                const machines = faker.helpers.multiple(
                  (_, i) =>
                    createCreatedEvent<InfraMachineSpec>(
                      {
                        spec: {},
                        metadata: {
                          id: faker.string.uuid(),
                          labels: { [LabelInfraProviderID]: 'bare-metal' },
                        },
                      },
                      i + 1,
                    ),
                  { count: limit },
                )

                ;[...machines, createBootstrapEvent(100)].forEach((event) =>
                  controller.enqueue(encodeResponse(event)),
                )
              },
            })

            return new HttpResponse(stream, {
              headers: {
                'content-type': 'application/json',
                'Grpc-metadata-content-type': 'application/grpc',
              },
            })
          },
        ),
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, WatchRequest>(
          '/omni.resources.ResourceService/Watch',
          async ({ request }) => {
            const { type, namespace } = await request.json()

            if (type !== InfraMachineType || namespace !== InfraProviderNamespace) return

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                controller.enqueue(encodeResponse(createBootstrapEvent()))
              },
            })

            return new HttpResponse(stream, {
              headers: {
                'content-type': 'application/json',
                'Grpc-metadata-content-type': 'application/grpc',
              },
            })
          },
        ),
      ],
    },
  },
}
