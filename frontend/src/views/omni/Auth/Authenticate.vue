<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { User } from '@auth0/auth0-spa-js'
import type { Auth0VueClient } from '@auth0/auth0-vue'
import { useAuth0 } from '@auth0/auth0-vue'
import { jwtDecode } from 'jwt-decode'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { fetchOption } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import { withMetadata } from '@/api/options'
import {
  authBearerHeaderPrefix,
  AuthFlowQueryParam,
  authHeader,
  authPublicKeyIDQueryParam,
  CLIAuthFlow,
  FrontendAuthFlow,
  RedirectQueryParam,
  samlSessionHeader,
  SignedRedirect,
  WorkloadProxyAuthFlow,
  workloadProxyPublicKeyIdSignatureBase64Cookie,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'
import { AuthType, authType } from '@/methods'
import { useLogout } from '@/methods/auth'
import { useIdentity } from '@/methods/identity'
import { createKeys, useKeys, useSignDetached } from '@/methods/key'
import { showError } from '@/notification'

const user = ref<User | undefined>(undefined)
let idToken = ''

/** @description Redirect the user's top-most window to the given URL.
 *
 * This makes sure that the redirect works correctly when the call comes from inside an iframe.
 *
 * @param url The URL to redirect to.
 */
const redirectToURL = (url: string) => {
  if (window.top) {
    window.top.location.href = url
  } else {
    window.location.href = url
  }
}

let auth0: Auth0VueClient | undefined

onMounted(() => {
  switch (authType.value) {
    case AuthType.Auth0:
      auth0 = useAuth0()

      user.value = auth0.user.value
      idToken = auth0.idTokenClaims.value!.__raw

      break
    case AuthType.OIDC:
    case AuthType.SAML:
      const navigateToLogin = () => {
        redirectToURL(`/login${window.location.search}`)
      }

      if (!identity.value) {
        navigateToLogin()
      }
  }
})

const authenticated = computed(() => {
  return identity.value
})

const identity = computed(() => {
  switch (authType.value) {
    case AuthType.Auth0:
      return user.value?.email
    case AuthType.SAML:
      return route.query.identity as string
    case AuthType.OIDC:
      return tokenData.value?.email
  }

  return undefined
})

const route = useRoute()

const tokenData = ref<{
  email?: string
  picture?: string
  name?: string
}>(route.query.token ? jwtDecode(route.query.token as string) : {})

const name = computed(() => {
  switch (authType.value) {
    case AuthType.Auth0:
      return user.value?.name
    case AuthType.OIDC:
      return tokenData.value.name
    case AuthType.SAML:
      return (route.query.fullname ?? route.query.identity) as string
  }

  return ''
})

const picture = computed(() => {
  switch (authType.value) {
    case AuthType.Auth0:
      return user?.value?.picture
    case AuthType.OIDC:
      return tokenData.value.picture
  }

  return undefined
})

const router = useRouter()

let publicKeyId = route.query[authPublicKeyIDQueryParam] as string

const confirmed = ref(false)

let redirect: string = route.query[RedirectQueryParam] as string

const logout = useLogout()
const keys = useKeys()
const identityStorage = useIdentity()

const generatePublicKey = async () => {
  if (!identity.value) {
    return
  }

  const res = await createKeys(identity.value)

  publicKeyId = res.publicKeyId

  try {
    await confirmPublicKey(res.privateKey)
  } catch {
    return
  }

  keys.privateKeyArmored.value = res.privateKey
  keys.publicKeyArmored.value = res.publicKey
  keys.publicKeyID.value = publicKeyId

  identityStorage.identity.value = identity.value.toLowerCase()
  identityStorage.fullname.value = name.value ?? ''
  identityStorage.avatar.value = picture.value ?? ''

  if (!redirect) {
    return
  }

  if (redirect.indexOf(SignedRedirect) === 0) {
    redirectToURL(`/exposed/service?${RedirectQueryParam}=${encodeURIComponent(redirect)}`)

    return
  }

  if (redirect.indexOf('/') !== 0) {
    redirect = '/'
  }

  await router.replace({ path: redirect })
}

let renewIdToken = false
const signDetached = useSignDetached()

const confirmPublicKey = async (privateKey?: string) => {
  try {
    if (renewIdToken && auth0) {
      renewIdToken = false

      await auth0.checkSession({
        cacheMode: 'off',
      })

      user.value = auth0.user.value
      idToken = auth0.idTokenClaims.value!.__raw
    }

    const options: fetchOption[] = []

    const metadata: Record<string, string> = {}

    if (privateKey) {
      const publicKeyIdSignatureBase64 = await signDetached(publicKeyId, privateKey)

      metadata[workloadProxyPublicKeyIdSignatureBase64Cookie] = publicKeyIdSignatureBase64
    }

    if (authType.value === AuthType.Auth0) {
      metadata[authHeader] = authBearerHeaderPrefix + idToken
    } else if (authType.value === AuthType.OIDC) {
      metadata[authHeader] = authBearerHeaderPrefix + route.query.token
    } else if (authType.value === AuthType.SAML) {
      if (!route.query.session) {
        throw new Error('no session')
      }

      metadata[samlSessionHeader] = route.query.session as string
    }

    options.push(withMetadata(metadata))

    await AuthService.ConfirmPublicKey(
      {
        public_key_id: publicKeyId,
      },
      ...options,
    )

    confirmed.value = true
  } catch (e) {
    showError('Failed to confirm public key', e.message)

    if (e?.code === Code.UNAUTHENTICATED && auth0) {
      renewIdToken = true
    }

    throw e
  }
}

const Auth = {
  CLI: CLIAuthFlow,
  Frontend: FrontendAuthFlow,
  WorkloadProxy: WorkloadProxyAuthFlow,
}
</script>

<template>
  <div class="flex h-full items-center justify-center">
    <div class="flex flex-col gap-2 rounded-md bg-naturals-n3 px-8 py-8 drop-shadow-md">
      <div class="flex items-center gap-4">
        <TIcon icon="key" class="fill-color h-6 w-6" />
        <div class="text-xl font-bold text-naturals-n13">
          <div v-if="$route.query[AuthFlowQueryParam] === Auth.CLI">Authenticate CLI Access</div>
          <div v-else-if="$route.query[AuthFlowQueryParam] === Auth.Frontend">
            Authenticate UI Access
          </div>
          <div v-else-if="$route.query[AuthFlowQueryParam] === Auth.WorkloadProxy">
            Authenticate Workload Proxy Access
          </div>
        </div>
      </div>

      <div v-if="!publicKeyId && $route.query[AuthFlowQueryParam] === Auth.CLI" class="mx-12">
        Public key ID parameter is missing...
      </div>
      <template v-else>
        <div v-if="!authenticated">Redirecting to the authentication provider...</div>
        <div v-else-if="!identity">
          Could not get user's email address from the authentication provider...
        </div>
        <template v-else>
          <div v-if="confirmed" id="confirmed">
            Successfully logged in as {{ identity }}, you can return to the application...
          </div>
          <div v-else class="flex w-full flex-col gap-4">
            <div>The keys are going to be issued for the user:</div>
            <UserInfo
              user="user"
              class="rounded-md bg-naturals-n6 px-6 py-2"
              :email="identity"
              :avatar="picture"
              :fullname="name"
            />
            <div class="flex w-full flex-col gap-3">
              <TButton type="secondary" class="w-full" @click="logout">Switch User</TButton>
              <TButton
                v-if="$route.query[AuthFlowQueryParam] === Auth.CLI"
                id="confirm"
                class="w-full"
                type="highlighted"
                @click="() => confirmPublicKey()"
              >
                Grant Access
              </TButton>
              <TButton
                v-else
                id="login"
                class="w-full"
                type="highlighted"
                @click="generatePublicKey"
              >
                Log In
              </TButton>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>
