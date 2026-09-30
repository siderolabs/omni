<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineSlots<{
  head(): unknown
  body(): unknown
}>()
</script>

<template>
  <table class="text-xs text-content-default">
    <thead v-if="$slots.head" class="bg-surface-card text-left">
      <slot name="head"></slot>
    </thead>

    <tbody v-if="$slots.body" class="[&_tr]:border-t">
      <slot name="body"></slot>
    </tbody>
  </table>
</template>
