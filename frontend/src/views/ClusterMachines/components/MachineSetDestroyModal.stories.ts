// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'

import type { DeleteRequest, DeleteResponse } from '@/api/omni/resources/resources.pb.ts'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, ResourceManagedByClusterTemplates } from '@/api/resources'

import MachineSetDestroyModal from './MachineSetDestroyModal.vue'

faker.seed(0)

const clusterId = faker.helpers.slugify(faker.word.words(3).toLowerCase())
const machineSetId = faker.helpers.slugify(faker.word.words(3).toLowerCase())

const meta: Meta<typeof MachineSetDestroyModal> = {
  component: MachineSetDestroyModal,
  args: {
    open: true,
    clusterId,
    machineSetId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterSpec>({
          expectedOptions: {
            type: ClusterType,
            namespace: DefaultNamespace,
            id: clusterId,
          },
          initialResources: [
            {
              spec: {},
              metadata: {
                type: ClusterType,
                namespace: DefaultNamespace,
                id: clusterId,
                annotations: {
                  [ResourceManagedByClusterTemplates]: '',
                },
              },
            },
          ],
        }).handler,

        http.post<never, DeleteRequest, DeleteResponse>(
          '/omni.resources.ResourceService/Teardown',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),
      ],
    },
  },
}
