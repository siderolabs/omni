<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center justify-center">
    <div class="flex flex-col items-center">
      <div class="code">404</div>
      <div class="text-xl text-center">Page not found</div>
      <t-button class="mt-4" @click="() => $router.push({ path: '/' })">Go Back</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
</script>

<style scoped>
.code {
  @apply font-bold text-naturals-N2;
  font-size: 300px;
  line-height: 300px;
}
</style>
