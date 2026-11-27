<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{ label: string }>()
</script>

<template>
  <div class="flex items-start justify-between gap-compact px-compact py-tight text-xs">
    <dt class="shrink-0 text-content-secondary">{{ label }}</dt>
    <dd class="min-w-0 text-right wrap-anywhere text-content-default">
      <slot></slot>
    </dd>
  </div>
</template>
