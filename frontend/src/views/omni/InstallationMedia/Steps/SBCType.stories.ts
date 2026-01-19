// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { ListRequest, ListResponse } from '@/api/omni/resources/resources.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { SBCConfigType, VirtualNamespace } from '@/api/resources'

import SBCType from './SBCType.vue'

const meta: Meta<typeof SBCType> = {
  component: SBCType,
  args: {
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, ListRequest, ListResponse>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== SBCConfigType || namespace !== VirtualNamespace) return

            const items = faker.helpers.multiple<Resource<SBCConfigSpec>>(
              () => ({
                metadata: {
                  namespace: VirtualNamespace,
                  type: SBCConfigType,
                  id: faker.string.uuid(),
                },
                spec: {
                  label: faker.commerce.productName(),
                  documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
                  min_version: faker.helpers.maybe(
                    () =>
                      `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
                  ),
                },
              }),
              { count: 20 },
            )

            return HttpResponse.json({
              items: items.map((item) => JSON.stringify(item)),
              total: items.length,
            })
          },
        ),
      ],
    },
  },
} satisfies Story
