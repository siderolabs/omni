// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import RadioGroup from './RadioGroup.vue'
import RadioGroupOption from './RadioGroupOption.vue'

const meta: Meta<typeof RadioGroup> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: RadioGroup as any,
  subcomponents: { RadioGroupOption },
  args: {
    label: faker.company.name(),
    'onUpdate:modelValue': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { RadioGroup, RadioGroupOption },
    setup: () => ({ args }),
    template: `
      <RadioGroup v-bind="args">
        ${faker.helpers
          .uniqueArray(faker.commerce.isbn, 10)
          .map(
            (value) => `
          <RadioGroupOption value="${value}">
            ${faker.commerce.productName()}
            ${
              faker.helpers.maybe(
                () => `<template #description>${faker.commerce.productDescription()}</template>`,
              ) || ''
            }
          </RadioGroupOption>
        `,
          )
          .join('')}
      </RadioGroup>
    `,
  }),
}
