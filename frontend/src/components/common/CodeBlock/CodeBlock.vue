<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'

const { code = '' } = defineProps<{ code?: string }>()
</script>

<template>
  <div class="relative rounded border border-naturals-n7 bg-naturals-n2 text-naturals-n14">
    <div
      class="absolute top-2 right-2 z-10 flex items-center justify-center rounded-md p-1 backdrop-blur"
    >
      <CopyButton aria-label="Copy" :text="code" />
    </div>

    <div class="p-1">
      <pre class="overflow-auto px-3 py-1 font-mono text-xs/6 whitespace-pre">{{ code }}</pre>
    </div>
  </div>
</template>
