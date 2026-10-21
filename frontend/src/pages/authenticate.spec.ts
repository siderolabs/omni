// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import { fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { flushPromises } from '@vue/test-utils'
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
    RegisterPublicKey: vi.fn(),
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
  await waitFor(() =>
    expect(locationHrefSpy).toHaveBeenCalledExactlyOnceWith(`/login${expectedQueryString}`),
  )
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
      isLoading: ref(false),
      isAuthenticated: ref(true),
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

test('Waits for the provider to load before logging in', async () => {
  vi.clearAllMocks()

  authType.value = AuthType.Auth0

  const isLoading = ref(true)
  const loginWithRedirect = vi.fn()

  vi.mocked(useAuth0).mockReturnValue({
    isLoading,
    isAuthenticated: ref(false),
    user: ref(undefined),
    idTokenClaims: ref(undefined),
    loginWithRedirect,
  } as unknown as ReturnType<typeof useAuth0>)

  await router.push({ path: '/', query: { flow: 'frontend' } })
  await router.isReady()

  render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  expect(loginWithRedirect).not.toHaveBeenCalled()

  isLoading.value = false

  await waitFor(() =>
    expect(loginWithRedirect).toHaveBeenCalledExactlyOnceWith({
      appState: { target: '/?flow=frontend' },
      authorizationParams: undefined,
    }),
  )
})

// An unsolicited SAML response carries no flow, so a forged login must not skip the confirmation
test('Requires confirmation to log in without a frontend flow', async () => {
  vi.clearAllMocks()

  authType.value = AuthType.SAML

  vi.mocked(AuthService.RegisterPublicKey).mockResolvedValue({ public_key_id: 'key-id' })
  vi.mocked(AuthService.ConfirmPublicKey).mockResolvedValue({})

  await router.push({ path: '/', query: { session: 'session', identity: 'user@example.com' } })
  await router.isReady()

  mockLocation(router.currentRoute.value.fullPath.slice(1))

  render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  const logInButton = await screen.findByRole('button', { name: 'Log In' })

  await flushPromises()

  expect(AuthService.RegisterPublicKey).not.toHaveBeenCalled()
  expect(AuthService.ConfirmPublicKey).not.toHaveBeenCalled()

  await fireEvent.click(logInButton)

  await waitFor(() =>
    expect(AuthService.ConfirmPublicKey).toHaveBeenCalledExactlyOnceWith(
      { public_key_id: 'key-id' },
      expect.anything(),
    ),
  )
})

describe('Shows an error instead of logging in again when the identity provider gives no email', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  async function authenticate(query: Record<string, string>) {
    await router.push({ path: '/', query: { flow: 'frontend', ...query } })
    await router.isReady()

    const locationHrefSpy = mockLocation(router.currentRoute.value.fullPath.slice(1))

    render(Authenticate, {
      global: {
        plugins: [router],
      },
    })

    await waitFor(() =>
      expect(showError).toHaveBeenCalledExactlyOnceWith(
        'Failed to generate keys',
        'An email is required to issue keys',
      ),
    )

    return locationHrefSpy
  }

  test.each([
    { name: 'without an email claim', token: `header.${btoa(JSON.stringify({}))}.signature` },
    { name: 'that cannot be decoded', token: 'not-a-jwt' },
  ])('OIDC with a token $name', async ({ token }) => {
    authType.value = AuthType.OIDC

    vi.spyOn(console, 'warn').mockImplementation(() => {})

    const locationHrefSpy = await authenticate({ token })

    expect(locationHrefSpy).not.toHaveBeenCalled()
  })

  test('Auth0 with a user without an email', async () => {
    authType.value = AuthType.Auth0

    const loginWithRedirect = vi.fn()

    vi.mocked(useAuth0).mockReturnValue({
      isLoading: ref(false),
      isAuthenticated: ref(true),
      user: ref({}),
      idTokenClaims: ref({ __raw: 'token' }),
      loginWithRedirect,
    } as unknown as ReturnType<typeof useAuth0>)

    await authenticate({})

    expect(loginWithRedirect).not.toHaveBeenCalled()
  })
})
