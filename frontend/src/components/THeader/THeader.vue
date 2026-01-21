<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import OngoingTasks from '@/components/common/OngoingTasks/OngoingTasks.vue'
import { getDocsLink } from '@/methods'

interface Props {
  sidebarOpen?: boolean
  onToggleSidebar?(): void
}

const props = defineProps<Props>()

const docsLink = getDocsLink('omni')
</script>

<template>
  <header
    class="flex h-12 items-center justify-between border-b border-naturals-n4 bg-naturals-n1 px-3 md:h-13 md:px-6"
  >
    <div class="flex items-center gap-4">
      <TButton
        class="relative size-6 p-0! md:hidden"
        aria-controls="sidebar"
        :aria-expanded="sidebarOpen"
        @click="props.onToggleSidebar"
      >
        <TIcon
          aria-label="open sidebar"
          :aria-hidden="sidebarOpen"
          icon="hamburger"
          class="absolute inset-0 m-auto size-4 transition-all"
          :class="sidebarOpen ? 'rotate-90 opacity-0' : 'opacity-100'"
        />
        <TIcon
          aria-label="close sidebar"
          :aria-hidden="!sidebarOpen"
          icon="close"
          class="absolute inset-0 m-auto size-4 transition-all"
          :class="sidebarOpen ? 'opacity-100' : '-rotate-90 opacity-0'"
        />
      </TButton>

      <RouterLink to="/" class="flex items-center gap-1 text-lg text-naturals-n13 uppercase">
        <TIcon class="t-header-icon size-6" icon="logo" />
        <span class="font-bold">Sidero</span>
        <span>Omni</span>
      </RouterLink>
    </div>

    <div class="flex items-center gap-2 text-naturals-n11">
      <a
        :href="docsLink"
        target="_blank"
        rel="noopener noreferrer"
        class="flex gap-1 transition-colors hover:text-naturals-n14"
      >
        <TIcon class="size-4" icon="info" />
        <span class="truncate text-xs max-sm:hidden">Documentation</span>
      </a>

      <a
        href="https://github.com/siderolabs/omni/issues"
        target="_blank"
        rel="noopener noreferrer"
        class="flex gap-1 transition-colors hover:text-naturals-n14"
      >
        <TIcon class="size-4" icon="check-in-circle" />
        <span class="truncate text-xs max-sm:hidden">Report an issue</span>
      </a>

      <OngoingTasks />
    </div>
  </header>
</template>
