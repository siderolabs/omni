<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { OIDCService } from '@/api/omni/oidc/oidc.pb'
import TButton from '@/components/Button/TButton.vue'
import CopyButton from '@/components/CopyButton/CopyButton.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import UserInfo from '@/components/UserInfo/UserInfo.vue'
import { useIdentity } from '@/methods/identity'
import { useTitle } from '@/methods/title'
import { showError } from '@/notification'
import AuthenticateCard from '@/views/Authenticate/components/AuthenticateCard.vue'

definePage({
  name: 'OIDC Login',
  meta: { guard: 'keys' },
})

const route = useRoute()

const { avatar, fullname, identity } = useIdentity()

const authRequestId = computed(() => route.params.authRequestId)

const authCode = ref<string>()

const confirmOIDCRequest = async () => {
  try {
    const response = await OIDCService.Authenticate({
      auth_request_id: authRequestId.value,
    })

    if (response.redirect_url) {
      window.location.href = response.redirect_url!

      return
    }

    authCode.value = response.auth_code
  } catch (e) {
    showError('Failed to confirm authenticate request', e instanceof Error ? e.message : String(e))
  }
}

useTitle('OIDC Login')
</script>

<template>
  <PageContainer class="flex h-full items-center justify-center">
    <AuthenticateCard title="Authenticate Kubernetes Access" icon="kubernetes">
      <div v-if="!authRequestId" class="mx-section">Auth request ID parameter is missing...</div>
      <div v-else class="flex w-full flex-col gap-compact">
        <p>The Kubernetes access is going to be granted for the user:</p>

        <UserInfo
          user="user"
          class="rounded-md bg-surface-inert px-base py-tight"
          :avatar="avatar"
          :fullname="fullname"
          :email="identity"
        />

        <div
          v-if="authCode"
          class="flex w-full items-center gap-tight rounded-lg border border-border-default px-snug py-micro"
        >
          <div class="grow text-sm text-content-emphasis">Access Code</div>

          <code
            class="rounded-md bg-surface-inert px-tight py-micro font-bold text-content-emphasis"
          >
            {{ authCode }}
          </code>

          <CopyButton :text="authCode" aria-label="Copy access code" />
        </div>

        <TButton v-else class="my-micro" variant="highlighted" @click="confirmOIDCRequest">
          Grant Access
        </TButton>
      </div>
    </AuthenticateCard>
  </PageContainer>
</template>
