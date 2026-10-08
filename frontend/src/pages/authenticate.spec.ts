// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import { fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import { AuthType, authType, requireReauthForNewKeys } from '@/methods'
import { showError } from '@/notification'

import Authenticate from './authenticate.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    {
      path: '/',
      component: { template: '<RouterView />' },
    },
  ],
})

vi.mock(import('@/methods'), async (importOriginal) => {
  const original = await importOriginal()

  return {
    ...original,
    authType: ref(original.AuthType.SAML),
  }
})

vi.mock('@auth0/auth0-vue', () => ({
  useAuth0: vi.fn(),
}))

vi.mock('@/api/omni/auth/auth.pb', () => ({
  AuthService: {
    ConfirmPublicKey: vi.fn(),
  },
}))

vi.mock('@/notification', () => ({
  showError: vi.fn(),
}))

function mockLocation(search: string) {
  const location = {
    href: `http://localhost:3000/${search}`,
    origin: 'http://localhost:3000',
    search,
  }

  Object.defineProperty(window, 'location', {
    value: location,
    writable: true,
  })

  // Spy on the href setter to verify the redirect URL
  return vi.spyOn(location, 'href', 'set')
}

test('Forwards query string for SAML auth', async () => {
  authType.value = AuthType.SAML

  await router.push({ path: '/', query: { thing: '+123cookies=', bacon: '@#_($%*#yes' } })
  await router.isReady()

  const expectedQueryString = '?thing=%2B123cookies=&bacon=@%23_($%25*%23yes'

  const locationHrefSpy = mockLocation(expectedQueryString)

  render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  expect(router.currentRoute.value.fullPath).toBe(`/${expectedQueryString}`)
  expect(locationHrefSpy).toHaveBeenCalledExactlyOnceWith(`/login${expectedQueryString}`)
})

describe('Retries the login once when the public key is rejected', () => {
  const token = `header.${btoa(JSON.stringify({ email: 'user@example.com' }))}.signature`

  const loginWithRedirect = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(AuthService.ConfirmPublicKey).mockRejectedValue(
      new RequestError('invalid jwt', { code: Code.UNAUTHENTICATED }),
    )

    vi.mocked(useAuth0).mockReturnValue({
      user: ref({ email: 'user@example.com' }),
      idTokenClaims: ref({ __raw: token }),
      loginWithRedirect,
    } as unknown as ReturnType<typeof useAuth0>)
  })

  async function grantAccess(query: Record<string, string>) {
    await router.push({ path: '/', query: { flow: 'cli', 'public-key-id': 'key-id', ...query } })
    await router.isReady()

    const locationHrefSpy = mockLocation(router.currentRoute.value.fullPath.slice(1))

    render(Authenticate, {
      global: {
        plugins: [router],
      },
    })

    await fireEvent.click(screen.getByRole('button', { name: 'Grant Access' }))

    return locationHrefSpy
  }

  test('OIDC logs in again without the rejected token', async () => {
    authType.value = AuthType.OIDC

    const locationHrefSpy = await grantAccess({ token })

    await waitFor(() =>
      expect(locationHrefSpy).toHaveBeenCalledExactlyOnceWith(
        '/login?flow=cli&public-key-id=key-id&retried=true',
      ),
    )
  })

  test('OIDC shows the error after a retry', async () => {
    authType.value = AuthType.OIDC

    const locationHrefSpy = await grantAccess({ token, retried: 'true' })

    await waitFor(() =>
      expect(showError).toHaveBeenCalledExactlyOnceWith(
        'Failed to confirm public key, please start the login again',
        'invalid jwt',
      ),
    )
    expect(locationHrefSpy).not.toHaveBeenCalled()
  })

  test.each([
    { requireReauth: true, maxAge: 0 },
    { requireReauth: false, maxAge: undefined },
  ])(
    'Auth0 logs in again with max_age $maxAge when requireReauthForNewKeys is $requireReauth',
    async ({ requireReauth, maxAge }) => {
      authType.value = AuthType.Auth0
      requireReauthForNewKeys.value = requireReauth

      await grantAccess({})

      await waitFor(() =>
        expect(loginWithRedirect).toHaveBeenCalledExactlyOnceWith({
          appState: { target: '/?flow=cli&public-key-id=key-id&retried=true' },
          authorizationParams: { max_age: maxAge },
        }),
      )
    },
  )

  test('Auth0 shows the error after a retry', async () => {
    authType.value = AuthType.Auth0

    await grantAccess({ retried: 'true' })

    await waitFor(() =>
      expect(showError).toHaveBeenCalledExactlyOnceWith(
        'Failed to confirm public key, please start the login again',
        'invalid jwt',
      ),
    )
    expect(loginWithRedirect).not.toHaveBeenCalled()
  })
})
