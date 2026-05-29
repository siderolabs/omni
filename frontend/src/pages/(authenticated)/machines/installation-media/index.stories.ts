// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { DeleteRequest, DeleteResponse } from '@/api/omni/resources/resources.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import {
  type JoinTokenStatusSpec,
  JoinTokenStatusSpecState,
} from '@/api/omni/specs/siderolink.pb.ts'
import { DefaultNamespace, InstallationMediaConfigType, JoinTokenStatusType } from '@/api/resources'
import * as DownloadPresetModalStories from '@/views/InstallationMedia/DownloadPresetModal.stories'

import InstallationMedia from './index.vue'

const meta: Meta<typeof InstallationMedia> = {
  component: InstallationMedia,
}

const joinTokens = faker.helpers.multiple(() => faker.string.alphanumeric(44), { count: 10 })

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        ...DownloadPresetModalStories.Default.parameters.msw.handlers,

        createWatchStreamHandler<JoinTokenStatusSpec>({
          expectedOptions: {
            type: JoinTokenStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: joinTokens.slice(1).map(
            (t, i) => ({
              spec: {
                is_default: i === 0,
                name: faker.word.noun(),
                state:
                  i === 0
                    ? JoinTokenStatusSpecState.ACTIVE
                    : faker.helpers.enumValue(JoinTokenStatusSpecState),
              },
              metadata: {
                id: t,
                type: JoinTokenStatusType,
                namespace: DefaultNamespace,
              },
            }),
            { count: 50 },
          ),
        }).handler,

        createWatchStreamHandler<InstallationMediaConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: InstallationMediaConfigType,
          },
          initialResources: faker.helpers.multiple(
            () => ({
              spec: {
                talos_version: faker.helpers.maybe(() => faker.system.semver()),
                join_token: faker.helpers.maybe(() => faker.helpers.arrayElement(joinTokens), {
                  probability: 0.75,
                }),
              },
              metadata: {
                id: faker.helpers.slugify(faker.word.words(3)),
                namespace: DefaultNamespace,
                type: InstallationMediaConfigType,
                created: faker.date.past().toISOString(),
              },
            }),
            { count: 50 },
          ),
        }).handler,

        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Delete',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== InstallationMediaConfigType || namespace !== DefaultNamespace) return

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
