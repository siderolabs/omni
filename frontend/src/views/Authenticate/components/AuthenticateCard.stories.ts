// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'
import { ref } from 'vue'

import { useIdentity } from '@/methods/identity.ts'
import { useKeys } from '@/methods/key.ts'
import type { AuthProvider } from '@/methods/useAuthProvider.ts'
import AuthenticateFrontend from '@/views/Authenticate/components/AuthenticateFrontend.vue'

import AuthenticateCard from './AuthenticateCard.vue'

const meta: Meta<typeof AuthenticateCard> = {
  component: AuthenticateCard,
  args: {
    title: 'Authenticate Access',
    icon: 'key',
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { AuthenticateCard },
    setup: () => ({ args }),
    template: '<AuthenticateCard v-bind="args">Content goes here</AuthenticateCard>',
  }),
}

export const FullCardExample: Story = {
  args: {
    title: 'Authenticate UI Access',
  },
  beforeEach({ msw }) {
    msw.use(
      http.post('/auth.AuthService/RegisterPublicKey', () =>
        HttpResponse.json({ public_key_id: faker.string.uuid() }),
      ),
      http.post('/auth.AuthService/ConfirmPublicKey', async () => {
        await delay()

        return HttpResponse.json({})
      }),
    )

    return () => {
      useKeys().clear()
      useIdentity().clear()
    }
  },
  render: (args) => ({
    components: { AuthenticateCard, AuthenticateFrontend },
    setup() {
      const provider: AuthProvider = {
        isLoading: ref(false),
        user: ref({
          email: faker.internet.email(),
          name: faker.person.fullName(),
          picture: faker.image.avatar(),
        }),
        canRetryLogin: false,
        credentials: () => ({}),
        login: fn(),
        logout: fn(),
      }

      return { args, provider }
    },
    template:
      '<AuthenticateCard v-bind="args"><AuthenticateFrontend :provider="provider" /></AuthenticateCard>',
  }),
}
