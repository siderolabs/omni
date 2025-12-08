<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'

const { code = '' } = defineProps<{ code?: string }>()
</script>

<template>
  <div
    class="relative rounded border border-naturals-n7 bg-naturals-n2 px-4 py-2 text-naturals-n14"
  >
    <div
      class="absolute top-2 right-2 z-10 flex items-center justify-center rounded-md p-1 backdrop-blur"
    >
      <CopyButton class="flex" aria-label="Copy" :text="code" />
    </div>

    <pre class="overflow-x-auto font-mono text-xs leading-6 whitespace-pre">{{ code }}</pre>
  </div>
</template>
