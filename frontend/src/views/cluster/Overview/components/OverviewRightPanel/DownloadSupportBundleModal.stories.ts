// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type {
  GetSupportBundleRequest,
  GetSupportBundleResponse,
} from '@/api/omni/management/management.pb'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterMachineIdentityType,
  ClusterPermissionsType,
  DefaultNamespace,
  LabelCluster,
  VirtualNamespace,
} from '@/api/resources'

import DownloadSupportBundleModal from './DownloadSupportBundleModal.vue'

const CLUSTER_ID = 'talos-default'
const NODE_IPS = faker.helpers.multiple(() => faker.internet.ipv4({ cidrBlock: '172.20.0.0/24' }))

const meta: Meta<typeof DownloadSupportBundleModal> = {
  component: DownloadSupportBundleModal,
  args: {
    open: true,
    clusterId: CLUSTER_ID,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ClusterPermissionsType || namespace !== VirtualNamespace) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { can_download_support_bundle: true },
              metadata: {
                namespace: VirtualNamespace,
                type: ClusterPermissionsType,
                id: CLUSTER_ID,
              },
            } as Resource<ClusterPermissionsSpec>),
          })
        }),

        createWatchStreamHandler<ClusterMachineIdentitySpec>({
          expectedOptions: {
            type: ClusterMachineIdentityType,
            namespace: DefaultNamespace,
            selectors: {
              [LabelCluster]: CLUSTER_ID,
            },
          },
          initialResources: NODE_IPS.map((ip) => ({
            spec: {
              node_ips: [ip],
              nodename: faker.helpers.slugify(
                [faker.hacker.adjective(), faker.hacker.verb(), faker.hacker.noun()]
                  .join(' ')
                  .toLowerCase(),
              ),
            },
            metadata: {
              type: ClusterMachineIdentityType,
              namespace: DefaultNamespace,
              id: faker.string.uuid(),
              labels: {
                [LabelCluster]: CLUSTER_ID,
              },
            },
          })),
        }).handler,

        http.post<never, GetSupportBundleRequest>(
          '/management.ManagementService/GetSupportBundle',
          () => {
            const stream = new ReadableStream<Uint8Array>({
              async start(c) {
                faker.seed(0)

                const enc = new TextEncoder()

                const sources = ['omni', 'cluster', ...NODE_IPS]

                const events = sources.flatMap((source) => {
                  const total = faker.number.int({ min: 30, max: 200 })

                  return faker.helpers.multiple<GetSupportBundleResponse>(
                    () => ({
                      progress: {
                        source,
                        total,
                        state: `collect ${faker.system.fileName({ extensionCount: { min: 1, max: 2 } })}`,
                      },
                    }),
                    { count: total },
                  )
                })

                for (const event of events) {
                  await delay(1)

                  c.enqueue(enc.encode(JSON.stringify(event) + '\n'))
                }

                c.enqueue(enc.encode(JSON.stringify({ bundle_data: 'foo' }) + '\n'))

                c.close()
              },
            })

            // GetSupportBundleResponse

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
