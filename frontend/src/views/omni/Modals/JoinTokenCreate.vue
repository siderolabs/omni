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
import TButtonGroup from '@/components/common/Button/TButtonGroup.vue'
import TNotification from '@/components/common/Notification/TNotification.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { createJoinToken } from '@/methods/user'
import type { Notification } from '@/notification'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

enum Lifetime {
  NeverExpire = 'Never Expire',
  Limited = 'Limited',
}

const notification: Ref<Notification | null> = shallowRef(null)

const expiration = ref(365)
const router = useRouter()

const lifetime: Ref<string> = ref(Lifetime.NeverExpire)

const key = ref<string>()
const name = ref<string>('')

const handleCreate = async () => {
  if (name.value === '') {
    showError('Failed to Create Join Token', 'Name is not defined', notification)

    return
  }

  try {
    await createJoinToken(
      name.value,
      lifetime.value === Lifetime.Limited ? expiration.value : undefined,
    )
  } catch (e) {
    showError('Failed to Create Join Token', e.message, notification)

    return
  }

  close()

  showSuccess('Join Token Was Created', undefined, notification)
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
      <h3 class="text-base text-naturals-N14">Create Join Token</h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex h-full w-full flex-col gap-4">
      <TNotification v-if="notification" v-bind="notification.props" />

      <template v-if="!key">
        <div class="flex flex-col gap-2">
          <TInput v-model="name" title="Name" class="h-full flex-1" :on-clear="() => (name = '')" />
          <div class="flex items-center gap-1 text-xs">
            Lifetime:
            <TButtonGroup
              v-model="lifetime"
              :options="[
                {
                  label: 'No Expiration',
                  value: Lifetime.NeverExpire,
                },
                {
                  label: 'Limited',
                  value: Lifetime.Limited,
                },
              ]"
            />
          </div>
          <TInput
            v-model="expiration"
            :disabled="lifetime === Lifetime.NeverExpire"
            title="Expiration Days"
            type="number"
            :min="1"
            class="flex-1"
          />
        </div>
        <TButton
          type="highlighted"
          :disabled="!canManageUsers && authType !== AuthType.SAML"
          class="h-9"
          @click="handleCreate"
          >Create Join Token</TButton
        >
      </template>
    </div>
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
