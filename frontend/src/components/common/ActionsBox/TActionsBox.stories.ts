// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TActionsBox from './TActionsBox.vue'
import TActionsBoxItem from './TActionsBoxItem.vue'

const meta: Meta<typeof TActionsBox> = {
  component: TActionsBox,
  subcomponents: { TActionsBoxItem },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { TActionsBox, TActionsBoxItem },
    setup: () => ({ args }),
    template: `
      <TActionsBox v-bind="args">
        <TActionsBoxItem icon="chart-bar">${faker.animal.cat()}</TActionsBoxItem>
        <TActionsBoxItem icon="dashboard">${faker.animal.cat()}</TActionsBoxItem>
        <TActionsBoxItem icon="delete" danger>${faker.animal.cat()}</TActionsBoxItem>
      </TActionsBox>`,
  }),
}
