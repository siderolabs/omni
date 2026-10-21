// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Decorator, Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'
import { markRaw, ref } from 'vue'
import { useRouter } from 'vue-router'

import type { Empty } from '@/api/google/protobuf/empty.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { ConfirmPublicKeyRequest } from '@/api/omni/auth/auth.pb'
import { authPublicKeyIDQueryParam } from '@/api/resources'
import type { AuthProvider } from '@/methods/useAuthProvider'

import AuthenticateCLI from './AuthenticateCLI.vue'

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

const query = { [authPublicKeyIDQueryParam]: faker.string.uuid() }

const withQuery: Decorator = (_story, { parameters }) => ({
  setup() {
    const ready = ref(false)

    useRouter()
      .replace({ path: '/authenticate', query: parameters.query })
      .then(() => (ready.value = true))

    // Prevent a flash by waiting for route change, a sstory only issue
    return { ready }
  },
  template: '<story v-if="ready" />',
})

const meta: Meta<typeof AuthenticateCLI> = {
  component: AuthenticateCLI,
  args: { provider },
  argTypes: { provider: { control: false } },
  parameters: {
    layout: 'centered',
  },
  decorators: [withQuery],
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: { query },
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
  parameters: { query },
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
  async play({ canvas, userEvent }) {
    await userEvent.click(await canvas.findByRole('button', { name: 'Grant Access' }))
  },
}

export const MissingPublicKeyID: Story = {}
