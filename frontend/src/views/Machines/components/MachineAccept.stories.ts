// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import MachineAccept from './MachineAccept.vue'

const meta: Meta<typeof MachineAccept> = {
  component: MachineAccept,
  args: {
    open: true,
    machines: faker.helpers.multiple(faker.string.uuid),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
