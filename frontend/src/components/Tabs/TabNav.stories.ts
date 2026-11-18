// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'
import { RouterView } from 'vue-router'

import TabNav from './TabNav.vue'

const routes = ['overview', 'monitor', 'logs', 'configuration', 'system'].map((path) => ({
  path: `/${path}`,
  component: RouterView,
}))

const meta: Meta<typeof TabNav> = {
  component: TabNav,
  decorators: [vueRouter(routes, { initialRoute: '/overview' })],
  args: {
    label: 'Machine',
    items: [
      { label: 'Overview', to: '/overview' },
      { label: 'Monitor', to: '/monitor' },
      { label: 'Logs', to: '/logs' },
      { label: 'Configuration', to: '/configuration', disabled: true },
      { label: 'System', to: '/system' },
    ],
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
