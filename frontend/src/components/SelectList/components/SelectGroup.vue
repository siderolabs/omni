<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SelectGroup, type SelectGroupProps, useForwardProps } from 'reka-ui'

const props = defineProps<SelectGroupProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <SelectGroup v-bind="forwarded">
    <slot></slot>
  </SelectGroup>
</template>
