<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref } from 'vue'
import { useRoute } from 'vue-router'

import { OIDCService } from '@/api/omni/oidc/oidc.pb'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'
import { useIdentity } from '@/methods/identity'
import { showError } from '@/notification'

const route = useRoute()
const { copy, copied } = useClipboard({ copiedDuring: 1000 })
const { avatar, fullname, identity } = useIdentity()

const authRequestId = route.params.authRequestId as string

const authCode = ref<string>()

const confirmOIDCRequest = async () => {
  try {
    const response = await OIDCService.Authenticate({
      auth_request_id: authRequestId,
    })

    if (response.redirect_url) {
      window.location.href = response.redirect_url!

      return
    }

    authCode.value = response.auth_code
  } catch (e) {
    showError('Failed to confirm authenticate request', e.message)

    throw e
  }
}

const copyCode = () => {
  if (authCode.value) copy(authCode.value)
}
</script>

<template>
  <div class="flex h-full items-center justify-center">
    <div class="flex flex-col gap-2 rounded-md bg-naturals-n3 px-8 py-8 drop-shadow-md">
      <div class="flex items-center gap-4">
        <TIcon icon="kubernetes" class="fill-color h-6 w-6" />
        <div class="text-xl font-bold text-naturals-n13">
          <div>Authenticate Kubernetes Access</div>
        </div>
      </div>

      <div v-if="!authRequestId" class="mx-12">Public key ID parameter is missing...</div>
      <template v-else>
        <div class="flex w-full flex-col gap-4">
          <div>The Kubernetes access is going to be granted for the user:</div>
          <UserInfo
            user="user"
            class="user-info"
            :avatar="avatar"
            :fullname="fullname"
            :email="identity"
          />
          <div
            v-if="authCode"
            class="flex w-full items-center justify-center gap-0.5 rounded-lg border border-naturals-n4 p-1 pl-2"
          >
            <div class="mr-2 text-sm text-naturals-n14">Access Code</div>
            <div class="flex-1" />
            <div
              class="cursor-pointer rounded-l-md bg-naturals-n6 px-2 py-0.5 font-mono font-bold text-naturals-n14"
              @click="copyCode"
            >
              {{ copied ? 'Copied' : authCode }}
            </div>
            <div
              class="cursor-pointer rounded-r-md bg-naturals-n6 px-2 py-1 text-naturals-n14 transition-colors hover:bg-naturals-n8"
              @click="copyCode"
            >
              <TIcon icon="copy" class="h-5" />
            </div>
          </div>
          <div v-else class="my-0.5 flex w-full flex-col gap-3">
            <TButton class="w-full" type="highlighted" @click="confirmOIDCRequest">
              Grant Access
            </TButton>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.user-info {
  @apply rounded-md bg-naturals-n6 px-6 py-2;
}
</style>
