// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { compare } from 'semver'

import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type {
  FeaturesConfigSpec,
  InstallationMediaSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import {
  APIConfigType,
  DefaultJoinTokenType,
  DefaultNamespace,
  EphemeralNamespace,
  FeaturesConfigID,
  FeaturesConfigType,
  InstallationMediaType,
  JoinTokenStatusType,
  TalosVersionType,
} from '@/api/resources'
import * as ExtensionsPickerStories from '@/views/omni/Extensions/ExtensionsPicker.stories.ts'

import DownloadInstallationMedia from './DownloadInstallationMedia.vue'

const joinTokens = faker.helpers.multiple(() => faker.string.alphanumeric(44), { count: 10 })

const meta: Meta<typeof DownloadInstallationMedia> = {
  component: DownloadInstallationMedia,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: FeaturesConfigType,
            id: FeaturesConfigID,
          },
          initialResources: [
            {
              spec: { image_factory_base_url: 'https://factory.talos.dev' },
              metadata: {
                namespace: DefaultNamespace,
                type: FeaturesConfigType,
                id: FeaturesConfigID,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<InstallationMediaSpec>({
          expectedOptions: {
            type: InstallationMediaType,
            namespace: EphemeralNamespace,
          },
          initialResources: faker.helpers.multiple(() => ({
            spec: { name: faker.animal.cat() },
            metadata: { id: faker.internet.domainName() },
          })),
        }).handler,

        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: faker.helpers
            .multiple(faker.system.semver, { count: 20 })
            .sort(compare)
            .map((id) => ({
              spec: {},
              metadata: { id },
            })),
        }).handler,

        createWatchStreamHandler<JoinTokenStatusSpec>({
          expectedOptions: {
            type: JoinTokenStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: joinTokens.map((name) => ({
            spec: { name },
            metadata: {},
          })),
        }).handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type } = await request.clone().json()

          if (type !== APIConfigType) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { enforce_grpc_tunnel: false },
              meta: {},
            }),
          })
        }),

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type } = await request.clone().json()

          if (type !== DefaultJoinTokenType) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { name: joinTokens[0] },
              meta: {},
            }),
          })
        }),

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type } = await request.clone().json()

          if (type !== JoinTokenStatusType) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { name: joinTokens[1] },
              meta: {},
            }),
          })
        }),

        http.post<never, GetRequest>('/management.ManagementService/CreateSchematic', () =>
          HttpResponse.json({
            pxe_url: faker.internet.url(),
            schematic_id: faker.string.uuid(),
          }),
        ),

        ...ExtensionsPickerStories.Data.parameters.msw.handlers,
      ],
    },
  },
}
