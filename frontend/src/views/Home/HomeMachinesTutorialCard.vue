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
    class="space-y-4 rounded-lg border border-primary-p3 bg-naturals-n2 p-6"
  >
    <header>
      <h2 class="text-sm font-medium text-naturals-n14">Getting Started: Machines</h2>
    </header>

    <div class="space-y-4 text-xs font-medium">
      <p>To add your first machine create Installation Media and put it onto your machine.</p>

      <p>
        Read
        <a
          class="link-primary"
          :href="
            getDocsLink(
              'omni',
              '/omni-cluster-setup/registering-machines/register-machines-with-omni',
            )
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Register machines with Omni
        </a>
        documentation to learn about other options.
      </p>
    </div>

    <div class="grid grid-cols-2 gap-2">
      <TButton icon="close" icon-position="left" @click="isDismissed = true">Dismiss</TButton>

      <TButton
        is="router-link"
        icon="long-arrow-down"
        icon-position="left"
        :to="{ name: 'InstallationMedia' }"
      >
        Download Installation Media
      </TButton>
    </div>
  </section>
</template>
