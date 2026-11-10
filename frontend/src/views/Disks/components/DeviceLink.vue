<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useScrollToDevice } from '@/views/Disks/lib/deviceNavigation'

defineProps<{
  id: string
  name: string
}>()

const scrollToDevice = useScrollToDevice()
</script>

<template>
  <button
    type="button"
    class="font-medium text-content-emphasis underline decoration-dotted underline-offset-2 hover:decoration-solid"
    @click="scrollToDevice(id)"
  >
    {{ name }}
  </button>
</template>
