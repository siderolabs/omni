<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { RedirectQueryParam } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import type { AuthProvider } from '@/methods/useAuthProvider'
import AuthenticateUser from '@/views/Authenticate/components/AuthenticateUser.vue'
import { useIssueBrowserKeys } from '@/views/Authenticate/lib/usePublicKeys'

const { provider, autoLogin } = defineProps<{
  provider: AuthProvider
  autoLogin?: boolean
}>()

const route = useRoute()
const router = useRouter()

const { issueKeys, issuing } = useIssueBrowserKeys(() => provider)

async function logIn() {
  if (!(await issueKeys())) return

  const redirect = route.query[RedirectQueryParam]?.toString()

  await router.replace({ path: redirect?.startsWith('/') ? redirect : '/' })
}

// The user is already signed in, so we log them in automatically.
// This is only attempted once, after which the user can retry with the button.
onMounted(() => autoLogin && logIn())
</script>

<template>
  <AuthenticateUser v-if="provider.user.value" :user="provider.user.value">
    <TButton variant="highlighted" :disabled="issuing" @click="logIn">
      <TSpinner v-if="issuing" class="size-4" />
      <template v-else>Log In</template>
    </TButton>
  </AuthenticateUser>
</template>
