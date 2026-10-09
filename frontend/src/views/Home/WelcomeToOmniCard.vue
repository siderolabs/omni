<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useLocalStorage } from '@vueuse/core'

import TButton from '@/components/Button/TButton.vue'
import { getDocsLink } from '@/methods'

const isDismissed = useLocalStorage('_home_machines_tutorial_dismissed', false)
</script>

<template>
  <section
    v-if="!isDismissed"
    class="space-y-compact rounded-lg border border-border-accent bg-surface-card p-base"
  >
    <header>
      <h2 class="text-sm font-medium text-content-emphasis">Welcome to Omni</h2>
    </header>

    <div class="space-y-compact text-xs font-medium">
      <p>
        Omni manages your Talos Linux clusters from one control plane. To get started, register your
        first machine — download installation media, boot a machine with it, and it'll appear here
        automatically.
      </p>

      <p>
        <a
          class="link-primary"
          :href="getDocsLink('omni', '/getting-started/getting-started')"
          target="_blank"
          rel="noopener noreferrer"
        >
          Follow the Getting Started guide →
        </a>
      </p>
    </div>

    <div class="flex items-center justify-end gap-tight">
      <TButton icon="close" icon-position="left" variant="secondary" @click="isDismissed = true">
        Dismiss
      </TButton>

      <TButton
        is="router-link"
        icon="long-arrow-down"
        icon-position="left"
        :to="{ name: 'InstallationMedia' }"
        variant="highlighted"
      >
        Download Installation Media
      </TButton>
    </div>
  </section>
</template>
