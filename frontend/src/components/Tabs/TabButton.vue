<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps, useForwardPropsEmits } from 'reka-ui'

// eslint-disable-next-line vue/define-props-destructuring
const props = defineProps<TabsTriggerProps>()
const forwarded = useForwardPropsEmits(props)
</script>

<template>
  <TabsTrigger
    v-bind="forwarded"
    :as="disabled ? 'button' : as"
    class="text-sm transition-colors not-data-disabled:hover:text-naturals-n13 data-disabled:cursor-not-allowed data-disabled:opacity-75 data-[state=active]:text-naturals-n13"
  >
    <slot></slot>
  </TabsTrigger>
</template>
