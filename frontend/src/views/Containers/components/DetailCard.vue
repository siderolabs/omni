<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{ title: string }>()
</script>

<template>
  <!-- The look of the machine Overview tab's cards: a heading over label/value rows -->
  <section class="rounded border border-border-default bg-surface-card">
    <h3 class="border-b border-border-default px-compact py-snug text-xs text-content-default">
      {{ title }}
    </h3>

    <dl class="flex flex-col py-micro">
      <slot></slot>
    </dl>
  </section>
</template>
