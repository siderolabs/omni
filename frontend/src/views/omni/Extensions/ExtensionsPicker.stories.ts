// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createBootstrapEvent, createCreatedEvent, encodeResponse } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { WatchRequest } from '@/api/omni/resources/resources.pb'
import type { TalosExtensionsSpec, TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, DefaultTalosVersion, TalosExtensionsType } from '@/api/resources'

import ExtensionsPicker from './ExtensionsPicker.vue'

faker.seed(0)
const fakeExtensions = faker.helpers.multiple(
  () => ({
    name: `siderolabs/${faker.helpers.slugify(faker.word.words({ count: { min: 1, max: 3 } }).toLowerCase())}`,
    author: faker.company.name(),
    version: faker.system.semver(),
    description: faker.lorem.sentences(4),
  }),
  { count: 50 },
) satisfies TalosExtensionsSpecInfo[]

const meta: Meta<typeof ExtensionsPicker> = {
  component: ExtensionsPicker,
  args: {
    talosVersion: DefaultTalosVersion,
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  args: {
    indeterminate: true,
    modelValue: {
      [fakeExtensions[0].name]: true,
      [fakeExtensions[1].name]: true,
    },
    immutableExtensions: {
      [fakeExtensions[1].name]: true,
    },
  },
  parameters: {
    msw: {
      handlers: [
        http.post<never, WatchRequest>(
          '/omni.resources.ResourceService/Watch',
          async ({ request }) => {
            const { id, type, namespace } = await request.json()

            if (
              id !== DefaultTalosVersion ||
              type !== TalosExtensionsType ||
              namespace !== DefaultNamespace
            ) {
              return
            }

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                ;[
                  createCreatedEvent<TalosExtensionsSpec>({
                    spec: {
                      items: fakeExtensions,
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
        http.post<never, WatchRequest>(
          '/omni.resources.ResourceService/Watch',
          async ({ request }) => {
            const { id, type, namespace } = await request.json()

            if (
              id !== DefaultTalosVersion ||
              type !== TalosExtensionsType ||
              namespace !== DefaultNamespace
            ) {
              return
            }

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
          },
        ),
      ],
    },
  },
}
