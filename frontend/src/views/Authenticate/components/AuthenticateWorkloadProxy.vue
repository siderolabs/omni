<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { RedirectQueryParam, SignedRedirect } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { type AuthProvider, redirectToURL } from '@/methods/useAuthProvider'
import AuthenticateUser from '@/views/Authenticate/components/AuthenticateUser.vue'
import { useIssueBrowserKeys } from '@/views/Authenticate/lib/usePublicKeys'

const { provider } = defineProps<{ provider: AuthProvider }>()

const route = useRoute()

const { issueKeys, issuing, confirmed } = useIssueBrowserKeys(() => provider)

async function logIn() {
  if (!(await issueKeys())) return

  const redirect = route.query[RedirectQueryParam]?.toString()

  if (redirect?.startsWith(SignedRedirect)) {
    redirectToURL(`/exposed/service?${RedirectQueryParam}=${encodeURIComponent(redirect)}`)
  }
}
</script>

<template>
  <p v-if="confirmed">
    Successfully logged in as {{ provider.user.value?.email }}, you can return to the application...
  </p>

  <AuthenticateUser v-else-if="provider.user.value" :user="provider.user.value">
    <TButton variant="highlighted" :disabled="issuing" @click="logIn">
      <TSpinner v-if="issuing" class="size-4" />
      <template v-else>Log In</template>
    </TButton>
  </AuthenticateUser>
</template>
