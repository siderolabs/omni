<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <TabList class="tabs">
    <slot />
  </TabList>
</template>

<script setup lang="ts">
import { TabList } from '@headlessui/vue'
</script>
