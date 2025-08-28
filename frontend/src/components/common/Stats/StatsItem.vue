<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon, { type IconType } from '@/components/common/Icon/TIcon.vue'

defineProps<{
  icon: IconType
  title: string
  value: string | number
}>()
</script>

<template>
  <div class="flex items-center gap-2">
    <TIcon class="size-5 text-primary-p2" aria-hidden="true" :icon="icon" />
    <span class="text-base whitespace-nowrap text-naturals-n14">{{ title }}: {{ value }}</span>
  </div>
</template>
