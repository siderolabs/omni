<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TNotification from '@/components/common/Notification/TNotification.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { renewServiceAccount } from '@/methods/user'
import type { Notification } from '@/notification'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

import ServiceAccountKey from './components/ServiceAccountKey.vue'

const notification: Ref<Notification | null> = shallowRef(null)

const router = useRouter()
const route = useRoute()

const key = ref<string>()

const expiration = ref(365)

const handleRenew = async () => {
  try {
    key.value = await renewServiceAccount(route.query.serviceAccount as string, expiration.value)
  } catch (e) {
    showError('Failed to Renew Service Account', e.message, notification)

    return
  }

  showSuccess('Service Account Key Was Renewed', undefined, notification)
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
      <h3 class="truncate text-base text-naturals-N14">
        Renew the Key for the Account {{ $route.query.serviceAccount }}
      </h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex h-full w-full flex-col gap-4">
      <TNotification v-if="notification" v-bind="notification.props" />

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
          >Generate New Key</TButton
        >
      </div>
    </div>

    <ServiceAccountKey v-if="key" :secret-key="key" />
  </div>
</template>

<style scoped>
.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-N2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-N14;
}

code {
  @apply break-all rounded bg-naturals-N4;
}
</style>
