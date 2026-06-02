// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb.ts'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb.ts'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources.ts'

import report from './sample-report.json'
import ScanDetailsModal from './ScanDetailsModal.vue'

const meta: Meta<typeof ScanDetailsModal> = {
  component: ScanDetailsModal,
  args: {
    open: true,
    schematicId: faker.string.uuid(),
    arch: PlatformConfigSpecArch.AMD64,
    talosVersion: '1.13.0',
  },
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: {
    matches: report.matches,
  },
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: FeaturesConfigType,
            id: FeaturesConfigID,
          },
          initialResources: [
            {
              spec: {
                image_factory_base_url: 'https://factory-enterprise.talos.dev',
              },
              metadata: {
                namespace: DefaultNamespace,
                type: FeaturesConfigType,
                id: FeaturesConfigID,
              },
            },
          ],
        }).handler,
      ],
    },
  },
}
