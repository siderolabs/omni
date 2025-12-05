// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { FeaturesConfigSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  FeaturesConfigType,
  JoinTokenStatusType,
  TalosVersionType,
} from '@/api/resources'

import TalosVersion from './TalosVersion.vue'

const meta: Meta<typeof TalosVersion> = {
  component: TalosVersion,
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
        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            type: FeaturesConfigType,
            namespace: DefaultNamespace,
          },
          initialResources: [
            {
              spec: { talos_pre_release_versions_enabled: true },
              metadata: {},
            },
          ],
        }).handler,

        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: faker.helpers
            .uniqueArray<string>(
              () =>
                `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
              40,
            )
            .concat(DefaultTalosVersion)
            .map((version) => ({
              spec: { version, deprecated: faker.datatype.boolean() },
              metadata: { id: version },
            })),
        }).handler,

        createWatchStreamHandler<JoinTokenStatusSpec>({
          expectedOptions: {
            type: JoinTokenStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: faker.helpers
            .uniqueArray(() => faker.string.alphanumeric(44), 10)
            .map((id) => ({
              metadata: { id },
              spec: {},
            })),
        }).handler,
      ],
    },
  },
} satisfies Story
