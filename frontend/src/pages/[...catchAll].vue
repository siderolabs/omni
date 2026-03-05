<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
</script>

<template>
  <PageContainer class="flex h-full items-center justify-center">
    <div class="flex flex-col items-center">
      <div class="code">404</div>
      <div class="text-center text-xl">Page not found</div>
      <TButton class="mt-4" @click="() => $router.push({ path: '/' })">Go Back</TButton>
    </div>
  </PageContainer>
</template>

<style scoped>
@reference "../index.css";

.code {
  @apply font-bold text-naturals-n2;
  font-size: 300px;
  line-height: 300px;
}
</style>
