// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { ListRequest } from '@/api/omni/resources/resources.pb'
import type { PlatformConfigSpec } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, VirtualNamespace } from '@/api/resources'

import MachineArch from './MachineArch.vue'

const meta: Meta<typeof MachineArch> = {
  component: MachineArch,
  args: {
    modelValue: { currentStep: 0 },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, ListRequest>(
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
                  secure_boot_supported: faker.datatype.boolean(),
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

export const ForCloud: Story = {
  args: {
    modelValue: {
      currentStep: 0,
      hardwareType: 'cloud',
    },
  },
}
