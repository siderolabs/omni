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
  <div class="flex items-center justify-between gap-1 rounded border border-naturals-n8 px-4 py-2">
    <pre class="font-mono text-xs whitespace-pre-wrap text-naturals-n11">{{ code }}</pre>
    <CopyButton class="shrink-0" :text="code" />
  </div>
</template>
