<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { splitImage } from '@/views/Containers/lib/containerStatus'

const { image, digest } = defineProps<{
  image?: string
  digest?: string
}>()

const parts = computed(() => (image ? splitImage(image) : undefined))
const tooltip = computed(() =>
  [image, digest !== image ? digest : undefined].filter(Boolean).join('\n'),
)
</script>

<template>
  <!-- The registry path gives way first, so the name and tag stay visible -->
  <span v-if="parts" class="flex min-w-0 font-mono" :title="tooltip">
    <span class="truncate text-content-secondary">{{ parts.path }}</span>
    <span class="shrink-0">{{ parts.name }}</span>
  </span>
  <span v-else>—</span>
</template>
