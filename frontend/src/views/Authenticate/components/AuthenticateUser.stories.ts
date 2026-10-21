// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TButton from '@/components/Button/TButton.vue'

import AuthenticateUser from './AuthenticateUser.vue'

faker.seed(0)

const meta: Meta<typeof AuthenticateUser> = {
  component: AuthenticateUser,
  args: {
    user: {
      email: faker.internet.email(),
      name: faker.person.fullName(),
      picture: faker.image.avatar(),
    },
  },
  parameters: {
    layout: 'centered',
  },
  render: (args) => ({
    components: { AuthenticateUser, TButton },
    setup: () => ({ args }),
    template:
      '<AuthenticateUser v-bind="args"><TButton variant="highlighted">Log in</TButton></AuthenticateUser>',
  }),
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
