// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'

import report from './sample-report.json'
import ScanDetailsModal from './ScanDetailsModal.vue'

const meta: Meta<typeof ScanDetailsModal> = {
  component: ScanDetailsModal,
  args: {
    open: true,
    schematicId: faker.string.uuid(),
    arch: PlatformConfigSpecArch.AMD64,
    factoryUrl: 'https://factory-enterprise.talos.dev',
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
}
