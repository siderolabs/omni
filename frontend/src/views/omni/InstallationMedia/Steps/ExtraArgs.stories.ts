// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { DefaultTalosVersion, SBCConfigType, VirtualNamespace } from '@/api/resources'

import ExtraArgs from './ExtraArgs.vue'

const SBC: Resource<SBCConfigSpec> = {
  metadata: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
    id: faker.string.uuid(),
  },
  spec: {
    label: faker.commerce.productName(),
    documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
    min_version: faker.helpers.maybe(
      () => `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
    ),
  },
}

const meta: Meta<typeof ExtraArgs> = {
  component: ExtraArgs,
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
        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== SBCConfigType || namespace !== VirtualNamespace) return

            return HttpResponse.json({
              body: JSON.stringify(SBC),
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
      sbcType: SBC.metadata.id,
    },
  },
}
