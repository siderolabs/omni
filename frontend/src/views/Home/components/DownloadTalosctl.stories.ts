// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { compare } from 'semver'

import type { Resource } from '@/api/grpc'
import type { ListRequest, ListResponse } from '@/api/omni/resources/resources.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { QuirksSpec } from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  QuirksType,
  TalosVersionType,
  VirtualNamespace,
} from '@/api/resources'
import type { TalosctlDownloadsResponse } from '@/methods/useTalosctlDownloads'

import DownloadTalosctl from './DownloadTalosctl.vue'

const meta: Meta<typeof DownloadTalosctl> = {
  component: DownloadTalosctl,
  args: {
    open: true,
  },
}

export default meta
type Story = StoryObj<typeof meta>

const versions = faker.helpers
  .uniqueArray<string>(
    () => `1.${faker.number.int({ min: 8, max: 13 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .concat(DefaultTalosVersion)
  .sort(compare)

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, ListRequest, ListResponse>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== QuirksType || namespace !== VirtualNamespace) return

            return HttpResponse.json({
              total: versions.length,
              items: versions.map((version) =>
                JSON.stringify({
                  metadata: {
                    namespace,
                    type,
                    id: version,
                  },
                  spec: {
                    supports_factory_talosctl: faker.datatype.boolean(),
                  },
                } satisfies Resource<QuirksSpec>),
              ),
            })
          },
        ),

        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: versions.map((version) => ({
            spec: { version, deprecated: faker.datatype.boolean() },
            metadata: { id: version },
          })),
        }).handler,

        http.get<{ version: string }>('/talosctl/downloads/:version', ({ params: { version } }) => {
          const downloads = faker.helpers
            .multiple(faker.hacker.noun, { count: 5 })
            .map(
              (name) =>
                `https://factory.talos.dev/talosctl/v${version}/talosctl-${name}-${faker.helpers.arrayElement(['amd64', 'arm64'])}`,
            )

          return HttpResponse.json<TalosctlDownloadsResponse>({
            status: '',
            downloads,
          })
        }),
      ],
    },
  },
}
