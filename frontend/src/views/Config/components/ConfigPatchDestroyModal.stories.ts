// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { DeleteRequest, DeleteResponse } from '@/api/omni/resources/resources.pb.ts'

import ConfigPatchDestroyModal from './ConfigPatchDestroyModal.vue'

faker.seed(0)

const patchId = `500-${faker.helpers.slugify(faker.word.words(3).toLowerCase())}`

const meta: Meta<typeof ConfigPatchDestroyModal> = {
  component: ConfigPatchDestroyModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    patchId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Delete',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
