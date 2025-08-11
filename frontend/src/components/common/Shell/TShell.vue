<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const isSidebarVisible = computed(() => {
  return route.path !== '/dashboard'
})
</script>

<template>
  <div class="shell">
    <div class="shell__menu" :class="{ 'shell__menu--hidden': !isSidebarVisible }">
      <slot name="menu"></slot>
    </div>
    <div class="shell__content">
      <slot name="content"></slot>
    </div>
  </div>
</template>

<style scoped>
.shell {
  @apply flex w-full bg-naturals-N0;
}
.shell__content {
  @apply flex w-full items-start justify-end overflow-y-auto px-2 px-6 py-6;
}
.shell__menu {
  transition: 0.4s ease all;
  position: relative;
}
.shell__menu--hidden {
  @apply absolute bottom-0 z-10;
  top: 52px;
  transform: translateX(-100%);
}
</style>
