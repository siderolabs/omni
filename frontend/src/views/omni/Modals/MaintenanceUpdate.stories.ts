// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createBootstrapEvent, createCreatedEvent, encodeResponse } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { WatchRequest } from '@/api/omni/resources/resources.pb'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'

import MaintenanceUpdate from './MaintenanceUpdate.vue'

faker.seed(0)
const talosVersions = faker.helpers.multiple<Resource<TalosVersionSpec>>(
  () => {
    const version = faker.system.semver()

    return {
      spec: { version, deprecated: faker.datatype.boolean() },
      metadata: { id: version },
    }
  },
  { count: 10 },
)

const meta: Meta<typeof MaintenanceUpdate> = {
  component: MaintenanceUpdate,
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
            const { type, namespace } = await request.clone().json()

            if (type !== TalosVersionType || namespace !== DefaultNamespace) {
              return
            }

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                ;[...talosVersions.map(createCreatedEvent), createBootstrapEvent(10)].forEach(
                  (event) => controller.enqueue(encodeResponse(event)),
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

        http.post<never, WatchRequest>(
          '/omni.resources.ResourceService/Watch',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== MachineStatusType || namespace !== DefaultNamespace) {
              return
            }

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                ;[
                  createCreatedEvent<MachineStatusSpec>({
                    spec: {
                      talos_version: `v${talosVersions.filter((s) => !s.spec.deprecated)[0].spec.version}`,
                    },
                    metadata: {},
                  }),
                  createBootstrapEvent(1),
                ].forEach((event) => controller.enqueue(encodeResponse(event)))
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
        http.post<never, WatchRequest>('/omni.resources.ResourceService/Watch', () => {
          const stream = new ReadableStream<Uint8Array>({
            start(controller) {
              ;[createBootstrapEvent(0)].forEach((event) =>
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
        }),
      ],
    },
  },
}
