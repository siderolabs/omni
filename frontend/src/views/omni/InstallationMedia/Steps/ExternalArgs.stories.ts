// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { ListRequest } from '@/api/omni/resources/resources.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { DefaultTalosVersion, SBCConfigType, VirtualNamespace } from '@/api/resources'

import ExternalArgs from './ExternalArgs.vue'

const SBCs = faker.helpers.multiple<Resource<SBCConfigSpec>>(
  () => ({
    metadata: {
      namespace: VirtualNamespace,
      type: SBCConfigType,
      id: faker.string.uuid(),
    },
    spec: {
      label: faker.commerce.productName(),
      documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
    },
  }),
  { count: 20 },
)

const meta: Meta<typeof ExternalArgs> = {
  component: ExternalArgs,
  args: {
    modelValue: { currentStep: 0, talosVersion: DefaultTalosVersion },
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

            if (type !== SBCConfigType || namespace !== VirtualNamespace) return

            return HttpResponse.json({
              items: SBCs.map((item) => JSON.stringify(item)),
              total: SBCs.length,
            })
          },
        ),
      ],
    },
  },
} satisfies Story

export const Pre1_10: Story = {
  ...Default,
  name: 'Pre-1.10',
  args: {
    modelValue: {
      currentStep: 0,
      secureBoot: false,
      talosVersion: '1.9.0',
    },
  },
}

export const WithSecureBoot: Story = {
  ...Default,
  args: {
    modelValue: {
      currentStep: 0,
      secureBoot: true,
    },
  },
}

export const WithOverlayOptions: Story = {
  ...Default,
  args: {
    modelValue: {
      currentStep: 0,
      talosVersion: DefaultTalosVersion,
      hardwareType: 'sbc',
      sbcType: SBCs[0].metadata.id,
    },
  },
}
