// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { milliseconds } from 'date-fns'
import { onMounted } from 'vue'
import type { ComponentProps } from 'vue-component-type-helpers'
import { toast } from 'vue-sonner'

import AppToast from './AppToast.vue'

type Props = ComponentProps<typeof AppToast>

const meta: Meta<Props & { type: 'error' | 'success' | 'warning' }> = {
  component: AppToast,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [() => ({ template: '<div class="h-screen"><story /></div>' })],
  render: (args) => ({
    setup() {
      onMounted(() => {
        toast[args.type](faker.word.noun(), {
          description: faker.word.words(10),
          duration: milliseconds({ hours: 1 }),
        })
      })

      return { args }
    },
    template: `<div />`,
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Error: Story = {
  args: {
    type: 'error',
  },
}

export const Success: Story = {
  args: {
    type: 'success',
  },
}

export const Warning: Story = {
  args: {
    type: 'warning',
  },
}
