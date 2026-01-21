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
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import * as DownloadPresetModalStories from '@/views/omni/InstallationMedia/DownloadPresetModal.stories'

import InstallationMedia from './InstallationMedia.vue'

const meta: Meta<typeof InstallationMedia> = {
  component: InstallationMedia,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        ...DownloadPresetModalStories.Default.parameters.msw.handlers,

        createWatchStreamHandler<InstallationMediaConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: InstallationMediaConfigType,
          },
          initialResources: faker.helpers.multiple(
            () => ({
              spec: {
                talos_version: faker.system.semver(),
              },
              metadata: {
                id: faker.helpers.slugify(faker.word.words(3)),
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
