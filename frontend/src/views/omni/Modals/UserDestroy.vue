<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ResourceService } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, IdentityType, UserType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const object = route.query.serviceAccount ? 'Service Account' : 'User'
const id = route.query.identity ?? route.query.serviceAccount

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const destroy = async () => {
  let destroyed = true

  if (route.query.serviceAccount) {
    const parts = (id as string).split('@')
    let name = parts[0]

    if (parts[1].indexOf('infra-provider') !== -1) {
      name = `infra-provider:${name}`
    }

    try {
      await ManagementService.DestroyServiceAccount({
        name,
      })
    } catch (e) {
      showError('Failed to Delete the Service Account', e.message)

      return
    }

    close()

    showSuccess(`Deleted Service Account ${id}`)

    return
  }

  if (route.query.user) {
    try {
      await ResourceService.Delete(
        {
          namespace: DefaultNamespace,
          type: UserType,
          id: route.query.user as string,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Remove the User', e.message)

        destroyed = false
      }
    }
  }

  if (route.query.identity) {
    try {
      await ResourceService.Delete(
        {
          namespace: DefaultNamespace,
          type: IdentityType,
          id: route.query.identity as string,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Remove the Identity', e.message)

        destroyed = false
      }
    }
  }

  close()

  if (destroyed) showSuccess(`The User ${route.query.identity} was Destroyed`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="flex-1 truncate text-base text-naturals-N14">
        Delete the {{ object }} {{ id }} ?
      </h3>
      <CloseButton @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" @click="destroy"> Delete </TButton>
    </div>
  </div>
</template>

<style scoped>
.heading {
  @apply mb-5 flex items-center gap-2 text-xl text-naturals-N14;
}
</style>
