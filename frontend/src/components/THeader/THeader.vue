<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <header class="t-header">
    <div class="t-header-nav-wrapper">
      <div class="t-header-logo-wrapper">
        <router-link to="/" class="flex items-center gap-1 text-lg text-naturals-N13 uppercase">
          <t-icon class="t-header-icon w-7 h-7" icon="logo" />
          <span class="font-bold">Sidero</span>
          <span>Omni</span>
        </router-link>
      </div>
      <div class="t-header-nav">
        <a href="https://omni.siderolabs.com/docs/" target="_blank" class="flex hover:text-naturals-N14">
          <t-icon class="t-header-nav-icon" icon="info" />
          <span class="t-header-nav-name">Documentation</span>
        </a>
        <a href="https://github.com/siderolabs/omni/issues" target="_blank" class="flex hover:text-naturals-N14">
          <t-icon class="t-header-nav-icon" icon="check-in-circle" />
          <span class="t-header-nav-name">Report an issue</span>
        </a>
        <ongoing-tasks/>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import TIcon from "@/components/common/Icon/TIcon.vue";
import OngoingTasks from "@/components/common/OngoingTasks/OngoingTasks.vue";
</script>

<style scoped>
.t-header {
  @apply px-6 py-3 flex justify-between items-center bg-naturals-N1 border-b border-naturals-N4;
  max-height: 52px;
}

.t-header-logo-wrapper {
  @apply mr-12 flex flex-1 self-center items-center font-firasans;
}

.t-header-nav-wrapper {
  @apply flex justify-start w-full;
}

.t-header-nav {
  @apply flex gap-2 text-naturals-N11 items-center justify-end;
}

.t-header-nav-icon {
  @apply w-4 h-4 mr-1;
}

.t-header-nav-name {
  @apply text-xs;
}
</style>
