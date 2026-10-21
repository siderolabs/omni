// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import { jwtDecode } from 'jwt-decode'
import { computed, type Ref, ref } from 'vue'
import { useRoute } from 'vue-router'

import {
  authBearerHeaderPrefix,
  authHeader,
  IdPLogoutPath,
  samlSessionHeader,
} from '@/api/resources'
import { AuthType, authType, requireReauthForNewKeys } from '@/methods'

export interface AuthUser {
  email?: string
  name?: string
  picture?: string
}

/**
 * AuthProvider is the identity provider's view of who is signed in, independent of the provider type.
 */
export interface AuthProvider {
  /** True while the provider does not yet know whether anyone is signed in. */
  isLoading: Readonly<Ref<boolean>>
  /** The signed in user, or undefined if nobody is signed in. */
  user: Readonly<Ref<AuthUser | undefined>>
  /** Whether a rejected login should be retried through the provider. */
  canRetryLogin: boolean
  /** The request metadata that proves the user's identity to Omni. */
  credentials(): Record<string, string>
  /**
   * Sends the user to the identity provider, which returns them to this page.
   *
   * @param query The query to come back with. Defaults to the current query as is.
   * @param retry Whether this is a retry after Omni rejected the previous login.
   */
  login(query?: URLSearchParams, retry?: boolean): Promise<void>
  /** Ends the user's session with the identity provider. */
  logout(): Maybe<Promise<void>>
}

/** @description Redirect the user's top-most window to the given URL.
 *
 * This makes sure that the redirect works correctly when the call comes from inside an iframe.
 *
 * @param url The URL to redirect to.
 */
export const redirectToURL = (url: string) => {
  if (window.top) {
    window.top.location.href = url
  } else {
    window.location.href = url
  }
}

export function useAuthProvider(): AuthProvider {
  switch (authType.value) {
    case AuthType.Auth0:
      return useAuth0Provider()
    case AuthType.OIDC:
      return useOIDCProvider()
    case AuthType.SAML:
      return useSAMLProvider()
    default:
      return useNoAuthProvider()
  }
}

function useAuth0Provider(): AuthProvider {
  const auth0 = useAuth0()
  const route = useRoute()

  return {
    isLoading: auth0.isLoading,
    user: computed(() => {
      const user = auth0.user.value

      if (!auth0.isAuthenticated.value || !user) return

      return { email: user.email, name: user.name, picture: user.picture }
    }),
    canRetryLogin: true,
    credentials() {
      const token = auth0.idTokenClaims.value?.__raw

      if (!token) throw new Error('no id token')

      return { [authHeader]: authBearerHeaderPrefix + token }
    },
    async login(query, retry) {
      await auth0.loginWithRedirect({
        appState: { target: query ? `${route.path}?${query}` : route.fullPath },
        authorizationParams: retry
          ? { max_age: requireReauthForNewKeys.value ? 0 : undefined }
          : undefined,
      })
    },
    async logout() {
      await auth0.logout({ logoutParams: { returnTo: window.location.origin } })
    },
  }
}

function useOIDCProvider(): AuthProvider {
  const route = useRoute()

  const token = computed(() => route.query.token?.toString())
  const claims = computed(() => {
    if (!token.value) return

    try {
      return jwtDecode<{ email?: string; name?: string; picture?: string }>(token.value)
    } catch (e) {
      console.warn('Failed to decode token', e)
      return {}
    }
  })

  return {
    isLoading: ref(false),
    user: computed(() => {
      if (!claims.value) return

      return { email: claims.value.email, name: claims.value.name, picture: claims.value.picture }
    }),
    canRetryLogin: true,
    credentials() {
      if (!token.value) throw new Error('no token')

      return { [authHeader]: authBearerHeaderPrefix + token.value }
    },
    async login(query) {
      query?.delete('token')

      redirectToLogin(query)
    },
    logout: redirectToLogout,
  }
}

function useSAMLProvider(): AuthProvider {
  const route = useRoute()

  return {
    isLoading: ref(false),
    user: computed(() => {
      if (!route.query.session) return

      const identity = route.query.identity?.toString()

      return { email: identity, name: route.query.fullname?.toString() || identity }
    }),
    canRetryLogin: false,
    credentials() {
      const session = route.query.session?.toString()

      if (!session) throw new Error('no session')

      return { [samlSessionHeader]: session }
    },
    async login(query) {
      query?.delete('session')
      query?.delete('identity')
      query?.delete('fullname')

      redirectToLogin(query)
    },
    logout: redirectToLogout,
  }
}

/** Used before the auth config is known, e.g. in storybook. Nobody is signed in. */
function useNoAuthProvider(): AuthProvider {
  return {
    isLoading: ref(false),
    user: ref(undefined),
    canRetryLogin: false,
    credentials() {
      throw new Error('no authentication provider is configured')
    },
    login() {
      throw new Error('no authentication provider is configured')
    },
    logout() {},
  }
}

/** The backend /login handler sends the user to the identity provider and back to /authenticate. */
function redirectToLogin(query?: URLSearchParams) {
  redirectToURL(query ? `/login?${query}` : `/login${window.location.search}`)
}

async function redirectToLogout() {
  redirectToURL(IdPLogoutPath)
}
