<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
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

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const isSidebarVisible = computed(() => {
  return route.path !== '/dashboard'
})
</script>

<style scoped>
.shell {
  @apply w-full flex bg-naturals-N0;
}
.shell__content {
  @apply flex justify-end items-start w-full px-2 py-6 px-6 overflow-y-auto;
}
.shell__menu {
  transition: 0.4s ease all;
  position: relative;
}
.shell__menu--hidden {
  @apply z-10 absolute bottom-0;
  top: 52px;
  transform: translateX(-100%);
}
</style>
