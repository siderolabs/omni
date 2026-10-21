// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'
import { markRaw, ref } from 'vue'

import type { Empty } from '@/api/google/protobuf/empty.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type {
  ConfirmPublicKeyRequest,
  RegisterPublicKeyRequest,
  RegisterPublicKeyResponse,
} from '@/api/omni/auth/auth.pb'
import { useIdentity } from '@/methods/identity'
import { useKeys } from '@/methods/key'
import type { AuthProvider } from '@/methods/useAuthProvider'

import AuthenticateFrontend from './AuthenticateFrontend.vue'

faker.seed(0)

// Args are made reactive, which would unwrap the provider's refs
const provider: AuthProvider = markRaw({
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
})

const meta: Meta<typeof AuthenticateFrontend> = {
  component: AuthenticateFrontend,
  args: { provider, autoLogin: true },
  argTypes: { provider: { control: false } },
  parameters: {
    layout: 'centered',
  },
  beforeEach({ msw }) {
    msw.use(
      http.post<never, RegisterPublicKeyRequest, RegisterPublicKeyResponse>(
        '/auth.AuthService/RegisterPublicKey',
        () => HttpResponse.json({ public_key_id: faker.string.uuid() }),
      ),
    )

    // Issuing the keys stores them for the frontend, which should not outlive the story
    return () => {
      useKeys().clear()
      useIdentity().clear()
    }
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.post<never, ConfirmPublicKeyRequest, Empty>(
        '/auth.AuthService/ConfirmPublicKey',
        async () => {
          await delay()

          return HttpResponse.json({})
        },
      ),
    )
  },
}

export const ConfirmError: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.post<never, ConfirmPublicKeyRequest>('/auth.AuthService/ConfirmPublicKey', async () => {
        await delay()

        return HttpResponse.json(
          { code: Code.PERMISSION_DENIED, message: 'public key is not allowed' },
          { status: 403 },
        )
      }),
    )
  },
}

export const Issuing: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.post<never, ConfirmPublicKeyRequest, Empty>(
        '/auth.AuthService/ConfirmPublicKey',
        async () => {
          await delay('infinite')

          return HttpResponse.json({})
        },
      ),
    )
  },
}
