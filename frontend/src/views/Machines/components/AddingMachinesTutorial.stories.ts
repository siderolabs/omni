// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb.ts'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources.ts'

import AddingMachinesTutorial from './AddingMachinesTutorial.vue'

const meta: Meta = {
  component: AddingMachinesTutorial,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<FeaturesConfigSpec>({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: FeaturesConfigType,
          id: FeaturesConfigID,
        },
        initialResources: [
          {
            metadata: {
              namespace: DefaultNamespace,
              type: FeaturesConfigType,
              id: FeaturesConfigID,
            },
            spec: {
              is_enterprise_image_factory: false,
            },
          },
        ],
      }).handler,
    )
  },
}

export const Enterprise: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<FeaturesConfigSpec>({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: FeaturesConfigType,
          id: FeaturesConfigID,
        },
        initialResources: [
          {
            metadata: {
              namespace: DefaultNamespace,
              type: FeaturesConfigType,
              id: FeaturesConfigID,
            },
            spec: {
              is_enterprise_image_factory: true,
              image_factory_base_url: 'https://factory-enterprise.talos.dev',
            },
          },
        ],
      }).handler,
    )
  },
}
