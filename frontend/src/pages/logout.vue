<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount } from 'vue'

import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { AuthType, authType } from '@/methods'
import { useLogout } from '@/methods/auth'

definePage({
  name: 'Logout',
})

const logout = useLogout()

onBeforeMount(() => {
  // For SAML and OIDC the backend registers its own /logout handler
  // Hard reload to hit backend forcibly if we are here accidentally
  if (authType.value !== AuthType.Auth0) window.location.reload()

  logout()
})
</script>

<template>
  <PageContainer class="flex h-full items-center justify-center">
    <div class="flex flex-col items-center gap-4">
      <TSpinner class="size-8" />
      <div class="text-xl text-naturals-n13">Logging out...</div>
    </div>
  </PageContainer>
</template>
