<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps, useForwardPropsEmits } from 'reka-ui'

import TabItem from './primitives/TabItem.vue'

const props = defineProps<TabsTriggerProps>()
const forwarded = useForwardPropsEmits(props)
</script>

<template>
  <TabItem>
    <TabsTrigger v-bind="forwarded">
      <slot></slot>
    </TabsTrigger>
  </TabItem>
</template>
