<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { authPublicKeyIDQueryParam } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import type { AuthProvider } from '@/methods/useAuthProvider'
import AuthenticateUser from '@/views/Authenticate/components/AuthenticateUser.vue'
import { useConfirmPublicKey } from '@/views/Authenticate/lib/usePublicKeys'

const { provider } = defineProps<{ provider: AuthProvider }>()

const route = useRoute()
const publicKeyId = computed(() => route.query[authPublicKeyIDQueryParam]?.toString())

const { confirmPublicKey, confirming, confirmed } = useConfirmPublicKey(() => provider)
</script>

<template>
  <p v-if="!publicKeyId" class="mx-section">Public key ID parameter is missing...</p>
  <p v-else-if="confirmed">
    Successfully logged in as {{ provider.user.value?.email }}, you can return to the application...
  </p>

  <AuthenticateUser v-else-if="provider.user.value" :user="provider.user.value">
    <TButton variant="highlighted" :disabled="confirming" @click="confirmPublicKey(publicKeyId)">
      <TSpinner v-if="confirming" class="size-4" />
      <template v-else>Grant Access</template>
    </TButton>
  </AuthenticateUser>
</template>
