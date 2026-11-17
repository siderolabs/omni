// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { vueRouter } from 'storybook-vue3-router'
import { RouterView } from 'vue-router'

import { Code } from '@/api/google/rpc/code.pb'
import type { AuthenticateRequest, AuthenticateResponse } from '@/api/omni/oidc/oidc.pb'
import { useIdentity } from '@/methods/identity'

import OIDCLogin from './[authRequestId].vue'

faker.seed(0)

const authRequestId = faker.string.uuid()

const withRoute = (initialRoute: string) =>
  vueRouter([{ path: '/:authRequestId?', component: RouterView }], { initialRoute })

const meta: Meta<typeof OIDCLogin> = {
  component: OIDCLogin,
  beforeEach() {
    const { identity, fullname, avatar, clear } = useIdentity()

    identity.value = faker.internet.email()
    fullname.value = faker.person.fullName()
    avatar.value = faker.image.avatar()

    // The identity lives in local storage, which should not outlive the story
    return clear
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  decorators: [withRoute(`/${authRequestId}`)],
  beforeEach({ msw }) {
    msw.use(
      http.post<never, AuthenticateRequest, AuthenticateResponse>(
        '/oidc.OIDCService/Authenticate',
        async () => {
          await delay()

          return HttpResponse.json({ auth_code: faker.string.alphanumeric(8).toUpperCase() })
        },
      ),
    )
  },
}

export const AccessCode: Story = {
  ...Default,
  async play({ canvas, userEvent }) {
    await userEvent.click(await canvas.findByRole('button', { name: 'Grant Access' }))
  },
}

export const AuthenticateError: Story = {
  decorators: [withRoute(`/${authRequestId}`)],
  beforeEach({ msw }) {
    msw.use(
      http.post<never, AuthenticateRequest>('/oidc.OIDCService/Authenticate', async () => {
        await delay()

        return HttpResponse.json(
          { code: Code.NOT_FOUND, message: 'auth request not found' },
          { status: 404 },
        )
      }),
    )
  },
  async play({ canvas, userEvent }) {
    await userEvent.click(await canvas.findByRole('button', { name: 'Grant Access' }))
  },
}

export const MissingAuthRequestID: Story = {
  decorators: [withRoute('/')],
}
