// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { ListRequest, ListResponse } from '@/api/omni/resources/resources.pb'
import { type PlatformConfigSpec, PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, VirtualNamespace } from '@/api/resources'

import CloudProvider from './CloudProvider.vue'

const meta: Meta<typeof CloudProvider> = {
  component: CloudProvider,
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

            if (type !== CloudPlatformConfigType || namespace !== VirtualNamespace) return

            const items = faker.helpers.multiple<Resource<PlatformConfigSpec>>(
              () => ({
                metadata: {
                  namespace: VirtualNamespace,
                  type: CloudPlatformConfigType,
                  id: faker.string.uuid(),
                },
                spec: {
                  label: faker.commerce.productName(),
                  description: faker.commerce.productDescription(),
                  documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
                  architectures: faker.helpers.arrayElements(
                    faker.helpers.uniqueArray(
                      () => faker.helpers.enumValue(PlatformConfigSpecArch),
                      2,
                    ),
                    { min: 1, max: 2 },
                  ),
                  secure_boot_supported: faker.datatype.boolean(),
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
