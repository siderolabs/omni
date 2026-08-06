<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount } from 'vue'

import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { useLogout } from '@/methods/auth'

definePage({
  name: 'Logout',
})

const logout = useLogout()

onBeforeMount(() => {
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
