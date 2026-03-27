<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useLocalStorage } from '@vueuse/core'

import TButton from '@/components/common/Button/TButton.vue'
import { getDocsLink } from '@/methods'

const isDismissed = useLocalStorage('_home_clusters_tutorial_dismissed', false)
</script>

<template>
  <section
    v-if="!isDismissed"
    class="space-y-4 rounded-lg border border-primary-p3 bg-naturals-n2 p-6"
  >
    <header>
      <h2 class="text-sm font-medium text-naturals-n14">Getting Started: Clusters</h2>
    </header>

    <div class="space-y-4 text-xs font-medium">
      <p>
        Read
        <a
          class="link-primary"
          :href="getDocsLink('omni', '/reference/cluster-templates')"
          target="_blank"
          rel="noopener noreferrer"
        >
          Cluster Templates
        </a>
        usage reference.
      </p>

      <p>
        Read
        <a
          class="link-primary"
          :href="getDocsLink('omni', '/getting-started/create-a-cluster')"
          target="_blank"
          rel="noopener noreferrer"
        >
          Create a Cluster
        </a>
        documentation to learn about other options.
      </p>
    </div>

    <div class="grid grid-cols-2 gap-2">
      <TButton icon="close" icon-position="left" @click="isDismissed = true">Dismiss</TButton>

      <TButton is="router-link" icon="plus" icon-position="left" :to="{ name: 'ClusterCreate' }">
        Create Cluster
      </TButton>
    </div>
  </section>
</template>
