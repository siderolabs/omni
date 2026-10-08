<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'

defineProps<{
  icon: IconType
  title: string
  value: string | number
}>()
</script>

<template>
  <div class="flex items-center gap-tight">
    <TIcon class="size-5 text-accent-hover" aria-hidden="true" :icon="icon" />
    <span class="text-base whitespace-nowrap text-content-emphasis">{{ title }}: {{ value }}</span>
  </div>
</template>
