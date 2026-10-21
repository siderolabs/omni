<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { until } from '@vueuse/core'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import {
  AuthFlowQueryParam,
  CLIAuthFlow,
  FrontendAuthFlow,
  WorkloadProxyAuthFlow,
} from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import { useTitle } from '@/methods/title'
import { useAuthProvider } from '@/methods/useAuthProvider'
import { showError } from '@/notification'
import AuthenticateCard from '@/views/Authenticate/components/AuthenticateCard.vue'
import AuthenticateCLI from '@/views/Authenticate/components/AuthenticateCLI.vue'
import AuthenticateFrontend from '@/views/Authenticate/components/AuthenticateFrontend.vue'
import AuthenticateWorkloadProxy from '@/views/Authenticate/components/AuthenticateWorkloadProxy.vue'

// For Auth0, the guard holds the navigation until Auth0 has handled its login callback and restored its session
definePage({
  name: 'Authenticate',
  meta: { guard: 'auth0' },
})

const route = useRoute()
const provider = useAuthProvider()

const flow = computed(() => route.query[AuthFlowQueryParam]?.toString())

// Only send the user to the identity provider once it is known that nobody is signed in,
// as the provider may still be restoring its session.
until(provider.isLoading)
  .toBe(false)
  .then(async () => {
    if (provider.user.value) return

    try {
      await provider.login()
    } catch (e) {
      showError('Error with provider log in', e instanceof Error ? e.message : String(e))
    }
  })

useTitle('Authenticate')

const title = computed(() => {
  switch (flow.value) {
    case CLIAuthFlow:
      return 'Authenticate CLI Access'
    case WorkloadProxyAuthFlow:
      return 'Authenticate Workload Proxy Access'
    case FrontendAuthFlow:
    default:
      return 'Authenticate UI Access'
  }
})
</script>

<template>
  <PageContainer class="flex h-full items-center justify-center">
    <AuthenticateCard :title="title" icon="key">
      <div v-if="!provider.user.value">Redirecting to the authentication provider...</div>
      <AuthenticateCLI v-else-if="flow === CLIAuthFlow" :provider />
      <AuthenticateWorkloadProxy v-else-if="flow === WorkloadProxyAuthFlow" :provider />
      <!-- We only auto login if flow is explicitly the Frontend flow -->
      <AuthenticateFrontend v-else :provider :auto-login="flow === FrontendAuthFlow" />
    </AuthenticateCard>
  </PageContainer>
</template>
