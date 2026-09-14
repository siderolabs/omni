<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SelectItemIndicator, type SelectItemIndicatorProps, useForwardProps } from 'reka-ui'

const props = defineProps<SelectItemIndicatorProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <SelectItemIndicator v-bind="forwarded">
    <slot></slot>
  </SelectItemIndicator>
</template>
