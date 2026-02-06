<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { TabsContent, type TabsContentProps, useForwardPropsEmits } from 'reka-ui'

const props = defineProps<TabsContentProps>()
const forwarded = useForwardPropsEmits(props)
</script>

<template>
  <TabsContent v-bind="forwarded">
    <slot></slot>
  </TabsContent>
</template>
