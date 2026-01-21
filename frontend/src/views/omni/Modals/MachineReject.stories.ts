// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'

import MachineReject from './MachineReject.vue'

const meta: Meta<typeof MachineReject> = {
  component: MachineReject,
}

export default meta
type Story = StoryObj<typeof meta>

const machines = faker.helpers.multiple(() => faker.string.uuid(), { count: 10 })
const query = new URLSearchParams(machines.map((m) => ['machine', m]))

export const Default: Story = {
  decorators: [
    vueRouter(undefined, {
      initialRoute: `/fake?${query}`,
    }),
  ],
}
