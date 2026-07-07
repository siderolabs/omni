<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'

import TButtonGroup from '@/components/Button/TButtonGroup.vue'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { createJoinToken } from '@/methods/user'
import { showError, showSuccess } from '@/notification'

const open = defineModel<boolean>('open', { default: false })

enum Lifetime {
  NeverExpire = 'Never Expire',
  Limited = 'Limited',
}

const expiration = ref(365)
const { canManageUsers } = usePermissions()

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

  open.value = false

  showSuccess('Join Token Was Created', undefined)
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Create Join Token"
    action-label="Create Join Token"
    :action-disabled="!canManageUsers && authType !== AuthType.SAML"
    @confirm="handleCreate"
  >
    <div class="flex flex-col gap-2">
      <TInput v-model="name" title="Name" class="h-full flex-1" clearable />

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
  </Modal>
</template>
