<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/Button/TButton.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import { useTitle } from '@/methods/title'

useTitle('Page Not Found')
</script>

<template>
  <PageContainer class="flex h-full flex-col items-center justify-center">
    <div class="text-[18.75rem] leading-none font-bold text-content-muted">404</div>
    <div class="text-center text-xl">Page not found</div>
    <TButton is="router-link" class="mt-4" :to="{ name: 'Home' }">Go Back</TButton>
  </PageContainer>
</template>
