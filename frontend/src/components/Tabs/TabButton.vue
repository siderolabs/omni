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
    :as="disabled ? 'button' : as"
    class="text-sm transition-colors not-data-disabled:hover:text-content-default data-disabled:cursor-not-allowed data-disabled:text-content-disabled data-[state=active]:text-content-default"
  >
    <slot></slot>
  </TabsTrigger>
</template>
