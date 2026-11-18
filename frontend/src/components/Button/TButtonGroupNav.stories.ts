// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'
import { RouterView } from 'vue-router'

import TButtonGroupNav from './TButtonGroupNav.vue'

const routes = ['current', 'patches', 'pending-updates', 'change-history'].map((path) => ({
  path: `/${path}`,
  component: RouterView,
}))

const meta: Meta<typeof TButtonGroupNav> = {
  component: TButtonGroupNav,
  decorators: [vueRouter(routes, { initialRoute: '/current' })],
  args: {
    label: 'Configuration',
    items: [
      { label: 'Current', to: '/current' },
      { label: 'Patches', to: '/patches' },
      { label: 'Pending Updates', to: '/pending-updates', disabled: true },
      { label: 'Change History', to: '/change-history' },
    ],
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
