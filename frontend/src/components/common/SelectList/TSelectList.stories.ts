// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import TSelectList from './TSelectList.vue'

const values = faker.helpers.uniqueArray(() => faker.animal.cat(), 100).sort()

const meta: Meta<typeof TSelectList> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: TSelectList as any,
  parameters: {
    layout: 'centered',
  },
  args: {
    title: 'Kitty',
    values,
    defaultValue: values.at(-Math.round(values.length / 2)),
    hideSelectedSmallScreens: false,
    searcheable: true,
    overheadTitle: false,
    onCheckedValue: fn(),
    'onUpdate:modelValue': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
