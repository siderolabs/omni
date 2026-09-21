// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { ComponentProps } from 'vue-component-type-helpers'

import UserInfo from './UserInfo.vue'

type Props = ComponentProps<typeof UserInfo>

faker.seed(0)

const meta: Meta<typeof UserInfo> = {
  component: UserInfo,
  args: {
    withLogoutControls: true,
    size: 'normal',
  },
  argTypes: {
    size: {
      control: 'inline-radio',
      options: ['normal', 'small'] satisfies Props['size'][],
    },
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: {
    email: 'john.doe@gmail.com',
    fullname: 'John Doe',
    avatar: faker.image.avatar(),
  },
}

export const NoAvatar: Story = {
  args: {
    email: 'john.doe@gmail.com',
    fullname: 'John Doe',
  },
}

export const OnlyEmail: Story = {
  args: {
    email: 'john.doe@gmail.com',
  },
}

export const MulticodeCharacters: Story = {
  args: {
    email: 'multicode.smith@gmail.com',
    fullname: '𠮷郎 Smith',
  },
}
