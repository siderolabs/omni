// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import TextArea from './TextArea.vue'

const meta: Meta<typeof TextArea> = {
  component: TextArea,
  args: {
    title: 'Title',
    placeholder: 'Placeholder',
    disabled: false,
    'onUpdate:modelValue': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
