<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'

defineProps<{
  title: string
  icon: IconType
}>()
</script>

<template>
  <div class="inline-flex flex-col gap-tight rounded-md bg-surface-raised p-section drop-shadow-md">
    <div class="flex items-center gap-compact text-content-default">
      <TIcon :icon="icon" class="size-6" />
      <span class="text-xl font-bold">{{ title }}</span>
    </div>

    <slot></slot>
  </div>
</template>
