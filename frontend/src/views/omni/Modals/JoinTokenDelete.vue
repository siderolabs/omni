<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { deleteJoinToken } from '@/methods/auth'
import { showError } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import JoinTokenWarnings from '@/views/omni/Modals/components/JoinTokenWarnings.vue'

const router = useRouter()
const route = useRoute()

const id = route.query.token as string

const isReady = ref(false)

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const deleteToken = async () => {
  try {
    await deleteJoinToken(id)
  } catch (e) {
    showError('Failed to Delete Token', e.message)
  }

  close()
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="flex-1 truncate text-base text-naturals-n14">Delete the token {{ id }} ?</h3>
      <CloseButton @click="close" />
    </div>

    <JoinTokenWarnings :id="id" class="mb-2 flex-1" @ready="isReady = true" />

    <p class="text-xs text-primary-p2">
      This action CANNOT be undone. This will permanently delete the Join Token.
    </p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="delete" :disabled="!isReady" @click="deleteToken">
        Delete
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center gap-2 text-xl text-naturals-n14;
}
</style>
