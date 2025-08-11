<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
import OngoingTasks from '@/components/common/OngoingTasks/OngoingTasks.vue'
</script>

<template>
  <header class="t-header">
    <div class="t-header-nav-wrapper">
      <div class="t-header-logo-wrapper">
        <RouterLink to="/" class="flex items-center gap-1 text-lg text-naturals-n13 uppercase">
          <TIcon class="t-header-icon h-7 w-7" icon="logo" />
          <span class="font-bold">Sidero</span>
          <span>Omni</span>
        </RouterLink>
      </div>
      <div class="t-header-nav">
        <a
          href="https://omni.siderolabs.com/"
          target="_blank"
          rel="noopener noreferrer"
          class="flex transition-colors hover:text-naturals-n14"
        >
          <TIcon class="t-header-nav-icon" icon="info" />
          <span class="t-header-nav-name max-sm:hidden">Documentation</span>
        </a>
        <a
          href="https://github.com/siderolabs/omni/issues"
          target="_blank"
          rel="noopener noreferrer"
          class="flex transition-colors hover:text-naturals-n14"
        >
          <TIcon class="t-header-nav-icon" icon="check-in-circle" />
          <span class="t-header-nav-name max-sm:hidden">Report an issue</span>
        </a>
        <OngoingTasks />
      </div>
    </div>
  </header>
</template>

<style scoped>
@reference "../../index.css";

.t-header {
  @apply flex items-center justify-between border-b border-naturals-n4 bg-naturals-n1 px-6 py-3;
  max-height: 52px;
}

.t-header-logo-wrapper {
  @apply mr-12 flex flex-1 items-center self-center font-firasans;
}

.t-header-nav-wrapper {
  @apply flex w-full justify-start;
}

.t-header-nav {
  @apply flex items-center justify-end gap-2 text-naturals-n11;
}

.t-header-nav-icon {
  @apply mr-1 h-4 w-4;
}

.t-header-nav-name {
  @apply truncate text-xs;
}
</style>
