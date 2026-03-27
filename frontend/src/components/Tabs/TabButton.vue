<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps, useForwardPropsEmits } from 'reka-ui'

const props = defineProps<TabsTriggerProps>()
const forwarded = useForwardPropsEmits(props)
</script>

<template>
  <TabsTrigger
    v-bind="forwarded"
    class="text-sm transition-colors hover:text-naturals-n13 data-[state=active]:text-naturals-n13"
  >
    <slot></slot>
  </TabsTrigger>
</template>
