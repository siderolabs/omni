// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecArch,
  PlatformConfigSpecBootMethod,
} from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, MetalPlatformConfigType, VirtualNamespace } from '@/api/resources'

import MachineArch from './MachineArch.vue'

const cloudProvider: Resource<PlatformConfigSpec> = {
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
}

const meta: Meta<typeof MachineArch> = {
  component: MachineArch,
  args: {
    modelValue: { hardwareType: 'metal' },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
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
                  label: 'Bare Metal',
                  description: faker.commerce.productDescription(),
                  documentation: '/talos-guides/install/bare-metal-platforms/',
                  architectures: [PlatformConfigSpecArch.AMD64, PlatformConfigSpecArch.ARM64],
                  secure_boot_supported: false,
                  boot_methods: [
                    PlatformConfigSpecBootMethod.DISK_IMAGE,
                    PlatformConfigSpecBootMethod.ISO,
                    PlatformConfigSpecBootMethod.PXE,
                  ],
                  disk_image_suffix: 'raw.zst',
                },
              } satisfies Resource<PlatformConfigSpec>),
            })
          },
        ),

        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== CloudPlatformConfigType || namespace !== VirtualNamespace) return

            return HttpResponse.json({
              body: JSON.stringify(cloudProvider),
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
      hardwareType: 'cloud',
      cloudPlatform: cloudProvider.metadata.id,
    },
  },
}
