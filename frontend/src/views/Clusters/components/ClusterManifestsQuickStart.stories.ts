// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import ClusterManifestsQuickStart from './ClusterManifestsQuickStart.vue'

const meta: Meta<typeof ClusterManifestsQuickStart> = {
  component: ClusterManifestsQuickStart,
}

export default meta
type Story = StoryObj<typeof ClusterManifestsQuickStart>

export const Default: Story = {}
