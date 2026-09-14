<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SelectViewport, type SelectViewportProps, useForwardProps } from 'reka-ui'

const props = defineProps<SelectViewportProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <SelectViewport v-bind="forwarded">
    <slot></slot>
  </SelectViewport>
</template>
