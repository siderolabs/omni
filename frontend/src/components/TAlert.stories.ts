// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'
import type { ComponentProps } from 'vue-component-type-helpers'

import TAlert from './TAlert.vue'

const types: ComponentProps<typeof TAlert>['type'][] = ['error', 'info', 'success', 'warn']

const meta: Meta<typeof TAlert> = {
  component: TAlert,
  argTypes: {
    type: {
      control: 'select',
      options: types,
    },
  },
  args: {
    type: 'info',
    title: 'Title',
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { TAlert },
    setup: () => ({ args }),
    template: `<TAlert v-bind="args">{{"${faker.hacker.phrase()}"}}</TAlert>`,
  }),
}

export const WithDismiss: Story = {
  args: {
    dismiss: {
      name: 'Dismiss',
      action: fn(),
    },
  },
  render: (args) => ({
    components: { TAlert },
    setup: () => ({ args }),
    template: `<TAlert v-bind="args">{{"${faker.hacker.phrase()}"}}</TAlert>`,
  }),
}

export const AllAlerts: Story = {
  decorators: [() => ({ template: '<div class="flex flex-col gap-2"><story/></div>' })],
  render: (args) => ({
    components: { TAlert },
    setup: () => ({ args }),
    template: types
      .map(
        (type) =>
          `<TAlert v-bind="args" title="${type[0].toUpperCase() + type.slice(1)}" type="${type}">{{"${faker.hacker.phrase()}"}}</TAlert>`,
      )
      .join(''),
  }),
}
