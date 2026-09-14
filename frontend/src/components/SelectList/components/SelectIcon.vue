<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SelectIcon, type SelectIconProps, useForwardProps } from 'reka-ui'

const props = defineProps<SelectIconProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <SelectIcon v-bind="forwarded">
    <slot></slot>
  </SelectIcon>
</template>
