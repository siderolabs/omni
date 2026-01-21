// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import '@/index.css'

import { createAuth0 } from '@auth0/auth0-vue'
import { milliseconds, millisecondsToSeconds } from 'date-fns'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import yamlWorker from 'monaco-yaml/yaml.worker?worker'
import { createApp } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { initState, ResourceService } from '@/api/grpc'
import type { AuthConfigSpec } from '@/api/omni/specs/auth.pb'
import { AuthConfigID, AuthConfigType, DefaultNamespace } from '@/api/resources'
import App from '@/App.vue'
import AppUnavailable from '@/AppUnavailable.vue'
import { AuthType, authType, suspended } from '@/methods'
import router from '@/router'

import { withRuntime } from './api/options'

if (import.meta.env.PROD) {
  self.MonacoEnvironment = {
    getWorker(_: string, label: string) {
      if (label === 'yaml') {
        return new yamlWorker()
      }

      return new editorWorker()
    },
  }
}

const setupApp = async () => {
  let authConfigSpec: AuthConfigSpec | undefined = undefined

  try {
    const authConfig: Resource<AuthConfigSpec> = await ResourceService.Get(
      {
        namespace: DefaultNamespace,
        type: AuthConfigType,
        id: AuthConfigID,
      },
      withRuntime(Runtime.Omni),
    )

    authConfigSpec = authConfig?.spec as AuthConfigSpec | undefined
  } catch (e) {
    console.error('failed to get auth parameters', e)
    createApp(AppUnavailable).mount('#app')
    return
  }

  suspended.value = authConfigSpec?.suspended ?? false

  if (authConfigSpec?.saml?.enabled) {
    authType.value = AuthType.SAML
  } else if (authConfigSpec?.oidc?.enabled) {
    authType.value = AuthType.OIDC
  } else if (authConfigSpec?.auth0?.enabled) {
    authType.value = AuthType.Auth0
  }

  let app = createApp(App).use(router)

  if (authType.value === AuthType.Auth0) {
    app = app.use(
      createAuth0({
        domain: authConfigSpec!.auth0!.domain!,
        clientId: authConfigSpec!.auth0!.client_id!,
        authorizationParams: {
          redirect_uri: window.location.origin,
          max_age: millisecondsToSeconds(milliseconds({ minutes: 2 })),
        },
        useFormData: !!authConfigSpec!.auth0?.useFormData,
      }),
    )
  }

  app.mount('#app')
}

initState()

setupApp()
