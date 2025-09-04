<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { User } from '@auth0/auth0-spa-js'
import type { Auth0VueClient } from '@auth0/auth0-vue'
import { useAuth0 } from '@auth0/auth0-vue'
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
  RedirectQueryParam,
  samlSessionHeader,
  SignedRedirect,
  WorkloadProxyAuthFlow,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'
import { AuthType, authType } from '@/methods'
import { createKeys, saveKeys } from '@/methods/key'
import { showError } from '@/notification'
import { FrontendAuthFlow } from '@/router'

const user = ref<User | undefined>(undefined)
let idToken = ''
let logout: (value: any) => void

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
  if (authType.value === AuthType.Auth0) {
    auth0 = useAuth0()

    user.value = auth0.user.value
    idToken = auth0.idTokenClaims.value!.__raw

    logout = async () => {
      if (auth0) {
        await auth0.logout({
          logoutParams: {
            returnTo: window.location.origin,
          },
        })
      }
    }
  } else {
    const navigateToLogin = () => {
      const query: string[] = []
      for (const key in route.query) {
        query.push(`${key}=${route.query[key]}`)
      }

      redirectToURL(`/login?${query.join('&')}`)
    }

    if (!identity.value) {
      navigateToLogin()
    }

    logout = () => navigateToLogin()
  }
})

const authenticated = computed(() => {
  return identity.value
})

const identity = computed(() => {
  if (authType.value === AuthType.Auth0) {
    return user.value?.email
  }

  if (authType.value === AuthType.SAML) {
    return route.query.identity as string
  }

  return undefined
})

const name = computed(() => {
  if (authType.value === AuthType.Auth0) {
    return user.value?.name
  }

  if (authType.value === AuthType.SAML) {
    return (route.query.fullname ?? route.query.identity) as string
  }

  return ''
})

const picture = computed(() => {
  return user?.value?.picture
})

const route = useRoute()
const router = useRouter()

let publicKeyId = route.query[authPublicKeyIDQueryParam] as string

const confirmed = ref(false)

let redirect: string = route.query[RedirectQueryParam] as string

const generatePublicKey = async () => {
  if (!identity.value) {
    return
  }

  const res = await createKeys(identity.value)

  publicKeyId = res.publicKeyId

  try {
    await confirmPublicKey()
  } catch {
    return
  }

  try {
    await saveKeys(
      {
        email: identity.value,
        fullname: name.value ?? '',
        picture: picture.value ?? '',
      },
      res.privateKey,
      res.publicKey,
      publicKeyId,
    )
  } catch (e) {
    showError('Failed to save the key', e.message)

    return
  }

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

const confirmPublicKey = async () => {
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

    if (authType.value === AuthType.Auth0) {
      options.push(withMetadata({ [authHeader]: authBearerHeaderPrefix + idToken }))
    } else if (authType.value === AuthType.SAML) {
      if (!route.query.session) {
        throw new Error('no session')
      }

      options.push(withMetadata({ [samlSessionHeader]: route.query.session as string }))
    }

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

const switchUser = () => {
  logout({ returnTo: window.location.origin })
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
            <UserInfo user="user" class="user-info" />
            <div class="flex w-full flex-col gap-3">
              <TButton type="secondary" class="w-full" @click="switchUser">Switch User</TButton>
              <TButton
                v-if="$route.query[AuthFlowQueryParam] === Auth.CLI"
                id="confirm"
                class="w-full"
                type="highlighted"
                @click="confirmPublicKey"
                >Grant Access</TButton
              >
              <TButton
                v-else
                id="login"
                class="w-full"
                type="highlighted"
                @click="generatePublicKey"
                >Log In</TButton
              >
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.user-info {
  @apply rounded-md bg-naturals-n6 px-6 py-2;
}
</style>
