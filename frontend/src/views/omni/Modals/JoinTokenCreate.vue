<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TButtonGroup from '@/components/common/Button/TButtonGroup.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { createJoinToken } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

enum Lifetime {
  NeverExpire = 'Never Expire',
  Limited = 'Limited',
}

const expiration = ref(365)
const router = useRouter()

const lifetime: Ref<string> = ref(Lifetime.NeverExpire)

const name = ref<string>('')

const handleCreate = async () => {
  if (name.value === '') {
    showError('Failed to Create Join Token', 'Name is not defined')

    return
  }

  try {
    await createJoinToken(
      name.value,
      lifetime.value === Lifetime.Limited ? expiration.value : undefined,
    )
  } catch (e) {
    showError('Failed to Create Join Token', e.message)

    return
  }

  close()

  showSuccess('Join Token Was Created', undefined)
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
    <div class="mb-5 flex items-center justify-between text-xl text-naturals-n14">
      <h3 class="text-base text-naturals-n14">Create Join Token</h3>
      <CloseButton @click="close" />
    </div>

    <div class="mb-4 flex flex-col gap-2">
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
      @click="handleCreate"
    >
      Create Join Token
    </TButton>
  </div>
</template>
