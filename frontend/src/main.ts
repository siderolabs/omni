// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import '@/index.css'

import { createAuth0 } from '@auth0/auth0-vue'
import { milliseconds, millisecondsToSeconds } from 'date-fns'
import { createApp } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { initState, ResourceService } from '@/api/grpc'
import type { AuthConfigSpec } from '@/api/omni/specs/auth.pb'
import { AuthConfigID, AuthConfigType, DefaultNamespace } from '@/api/resources'
import App from '@/App.vue'
import AppUnavailable, { appUnavailableError } from '@/AppUnavailable.vue'
import { AuthType, authType, suspended } from '@/methods'
import { registerPreloadListener } from '@/methods/registerPreloadListener'
import router from '@/router'

import { withRuntime, withTimeout } from './api/options'

async function setupApp() {
  let authConfig: Resource<AuthConfigSpec>

  try {
    authConfig = await ResourceService.Get<Resource<AuthConfigSpec>>(
      {
        namespace: DefaultNamespace,
        type: AuthConfigType,
        id: AuthConfigID,
      },
      withRuntime(Runtime.Omni),
      withTimeout(10_000),
    )
  } catch (e) {
    appUnavailableError.value = new Error(
      `Failed to load Omni configuration: ${e instanceof Error ? e.message : String(e)}`,
    )
    createApp(AppUnavailable).mount('#app')
    return
  }

  suspended.value = !!authConfig.spec.suspended

  const app = createApp(App).use(router)

  if (authConfig.spec.auth0?.enabled) {
    authType.value = AuthType.Auth0

    const searchParams = new URLSearchParams(window.location.search)

    // If there is any kind of error response from auth0, defensively assume signup isn't supported
    const signupDisabled = typeof searchParams.get('error') === 'string'

    const requestSignup = !authConfig.spec.has_initial_user && !signupDisabled

    app.use(
      createAuth0({
        domain: authConfig.spec.auth0.domain!,
        clientId: authConfig.spec.auth0.client_id!,
        authorizationParams: {
          redirect_uri: window.location.origin,
          max_age: millisecondsToSeconds(milliseconds({ minutes: 2 })),
          screen_hint: requestSignup ? 'signup' : 'login',
        },
        useFormData: !!authConfig.spec.auth0.useFormData,
      }),
    )
  } else if (authConfig.spec.saml?.enabled) {
    authType.value = AuthType.SAML
  } else if (authConfig.spec.oidc?.enabled) {
    authType.value = AuthType.OIDC
  }

  app.mount('#app')
}

registerPreloadListener()

initState()

setupApp()
