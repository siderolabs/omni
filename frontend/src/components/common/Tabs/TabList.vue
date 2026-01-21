<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { TabList } from '@headlessui/vue'
</script>

<template>
  <TabList class="tabs">
    <slot />
  </TabList>
</template>
