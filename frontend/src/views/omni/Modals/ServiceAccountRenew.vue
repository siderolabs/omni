<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { renewServiceAccount } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

import ServiceAccountKey from './components/ServiceAccountKey.vue'

const router = useRouter()
const route = useRoute()

const key = ref<string>()

const expiration = ref(365)

const handleRenew = async () => {
  try {
    key.value = await renewServiceAccount(route.query.serviceAccount as string, expiration.value)
  } catch (e) {
    showError('Failed to Renew Service Account', e.message)

    return
  }

  showSuccess('Service Account Key Was Renewed', undefined)
}

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="truncate text-base text-naturals-n14">
        Renew the Key for the Account {{ $route.query.serviceAccount }}
      </h3>
      <CloseButton @click="close" />
    </div>

    <div v-if="!key" class="flex flex-col gap-2">
      <TInput
        v-model="expiration"
        title="Expiration Days"
        type="number"
        :min="1"
        class="h-full flex-1"
      />
      <TButton
        type="highlighted"
        :disabled="!canManageUsers && authType !== AuthType.SAML"
        class="h-9"
        @click="handleRenew"
      >
        Generate New Key
      </TButton>
    </div>

    <ServiceAccountKey v-if="key" :secret-key="key" />
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-n2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}

code {
  @apply rounded bg-naturals-n4 break-all;
}
</style>
