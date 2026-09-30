<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref } from 'vue'
import { useRoute } from 'vue-router'

import { OIDCService } from '@/api/omni/oidc/oidc.pb'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import UserInfo from '@/components/UserInfo/UserInfo.vue'
import { useIdentity } from '@/methods/identity'
import { useTitle } from '@/methods/title'
import { showError } from '@/notification'

definePage({
  name: 'OIDC Login',
  meta: { guard: 'keys' },
})

const route = useRoute()

const { copy, copied } = useClipboard({ copiedDuring: 1000 })
const { avatar, fullname, identity } = useIdentity()

const authRequestId = route.params.authRequestId

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

useTitle('OIDC Login')
</script>

<template>
  <PageContainer class="flex h-full items-center justify-center">
    <div class="flex flex-col gap-2 rounded-md bg-surface-raised px-8 py-8 drop-shadow-md">
      <div class="flex items-center gap-4">
        <TIcon icon="kubernetes" class="fill-color h-6 w-6" />
        <div class="text-xl font-bold text-content-default">
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
            class="flex w-full items-center justify-center gap-0.5 rounded-lg border border-border-default p-1 pl-2"
          >
            <div class="mr-2 text-sm text-content-emphasis">Access Code</div>
            <div class="flex-1" />
            <div
              class="cursor-pointer rounded-l-md bg-surface-inert px-2 py-0.5 font-mono font-bold text-content-emphasis"
              @click="copyCode"
            >
              {{ copied ? 'Copied' : authCode }}
            </div>
            <div
              class="cursor-pointer rounded-r-md bg-surface-inert px-2 py-1 text-content-emphasis transition-colors hover:bg-surface-hover"
              @click="copyCode"
            >
              <TIcon icon="copy" class="h-5" />
            </div>
          </div>
          <div v-else class="my-0.5 flex w-full flex-col gap-3">
            <TButton class="w-full" variant="highlighted" @click="confirmOIDCRequest">
              Grant Access
            </TButton>
          </div>
        </div>
      </template>
    </div>
  </PageContainer>
</template>

<style scoped>
@reference "../../index.css";

.user-info {
  @apply rounded-md bg-surface-inert px-6 py-2;
}
</style>
