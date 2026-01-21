// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TMenuItem from './TMenuItem.vue'

const meta: Meta<typeof TMenuItem> = {
  component: TMenuItem,
  parameters: {
    layout: 'centered',
  },
  args: {
    name: 'Name',
    icon: 'kubernetes',
    label: 'Label',
    tooltip: 'Tooltip',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const WithSubItems: Story = {
  args: {
    name: 'Name',
    icon: 'kubernetes',
    label: 'Label',
    subItems: faker.helpers.multiple(() => ({ name: faker.animal.cat() }), { count: 5 }),
  },
}
