// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'

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
    // JSON imports merge the shapes of array elements, so the MatchDetail union can't be inferred
    matches: report.matches as Match[],
  },
}
