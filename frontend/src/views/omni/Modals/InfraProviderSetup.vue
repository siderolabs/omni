<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TNotification from '@/components/common/Notification/TNotification.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { setupInfraProvider } from '@/methods/providers'
import type { Notification } from '@/notification'
import { showError } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

import ServiceAccountKey from './components/ServiceAccountKey.vue'

const notification: Ref<Notification | null> = shallowRef(null)

const id = ref('')
const router = useRouter()

const key = ref<string>()

const handleCreate = async () => {
  if (id.value === '') {
    showError('Failed to Create Service Account', 'Name is not defined', notification)

    return
  }

  try {
    key.value = await setupInfraProvider(id.value)
  } catch (e) {
    showError('Failed to Create Service Account', e.message, notification)

    return
  }
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
      <h3 class="text-base text-naturals-n14">Setup a new Infra Provider</h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex h-full w-full flex-col gap-4">
      <TNotification v-if="notification" v-bind="notification.props" />

      <template v-if="!key">
        <div class="flex flex-col gap-2">
          <TInput
            v-model="id"
            title="Provider ID"
            class="h-full flex-1"
            placeholder="examples: kubevirt, bare-metal"
          />
        </div>
        <TButton type="highlighted" class="h-9" @click="handleCreate">Next</TButton>
      </template>
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
