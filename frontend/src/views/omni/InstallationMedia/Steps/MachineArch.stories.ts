// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, ListRequest } from '@/api/omni/resources/resources.pb'
import { type PlatformConfigSpec, PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, MetalPlatformConfigType, VirtualNamespace } from '@/api/resources'

import MachineArch from './MachineArch.vue'

const cloudProviders = faker.helpers.multiple<Resource<PlatformConfigSpec>>(
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
        faker.helpers.uniqueArray(() => faker.helpers.enumValue(PlatformConfigSpecArch), 2),
        { min: 1, max: 2 },
      ),
      secure_boot_supported: faker.datatype.boolean(),
      min_version: faker.helpers.maybe(
        () => `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
      ),
    },
  }),
  { count: 20 },
)

const meta: Meta<typeof MachineArch> = {
  component: MachineArch,
  args: {
    modelValue: { currentStep: 0, hardwareType: 'metal' },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== MetalPlatformConfigType || namespace !== VirtualNamespace) return

          return HttpResponse.json({
            body: JSON.stringify({
              metadata: {
                namespace: VirtualNamespace,
                type: MetalPlatformConfigType,
                id: faker.string.uuid(),
              },
              spec: {
                label: faker.commerce.productName(),
                description: faker.commerce.productDescription(),
                documentation: faker.system.directoryPath(),
                architectures: [PlatformConfigSpecArch.AMD64, PlatformConfigSpecArch.ARM64],
                secure_boot_supported: false,
              },
            } satisfies Resource<PlatformConfigSpec>),
          })
        }),

        http.post<never, ListRequest>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== CloudPlatformConfigType || namespace !== VirtualNamespace) return

            return HttpResponse.json({
              items: cloudProviders.map((item) => JSON.stringify(item)),
              total: cloudProviders.length,
            })
          },
        ),
      ],
    },
  },
} satisfies Story

export const ForCloud: Story = {
  ...Default,
  args: {
    modelValue: {
      currentStep: 0,
      hardwareType: 'cloud',
      cloudPlatform: cloudProviders[0].metadata.id,
    },
  },
}
