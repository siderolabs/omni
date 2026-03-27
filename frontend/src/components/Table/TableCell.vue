<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{ th?: boolean }>()
</script>

<template>
  <component :is="th ? 'th' : 'td'" class="px-2" :class="th ? 'py-2 uppercase' : 'py-2.5'">
    <slot></slot>
  </component>
</template>
