// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createBootstrapEvent, createCreatedEvent, encodeResponse } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { WatchRequest } from '@/api/omni/resources/resources.pb'
import type { KubernetesUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KubernetesUpgradeStatusType } from '@/api/resources'

import UpdateKubernetes from './UpdateKubernetes.vue'

const meta: Meta<typeof UpdateKubernetes> = {
  component: UpdateKubernetes,
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
            const { type, namespace } = await request.json()

            if (type !== KubernetesUpgradeStatusType || namespace !== DefaultNamespace) {
              return
            }

            const stream = new ReadableStream<Uint8Array>({
              start(controller) {
                faker.seed(0)

                const upgrade_versions = faker.helpers
                  .multiple(() => faker.system.semver(), { count: 10 })
                  .sort()

                const [last_upgrade_version] = upgrade_versions.splice(
                  Math.round(upgrade_versions.length / 2),
                  1,
                )

                ;[
                  createCreatedEvent<KubernetesUpgradeStatusSpec>({
                    spec: { last_upgrade_version, upgrade_versions },
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
            const { type, namespace } = await request.json()

            if (type !== KubernetesUpgradeStatusType || namespace !== DefaultNamespace) {
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
