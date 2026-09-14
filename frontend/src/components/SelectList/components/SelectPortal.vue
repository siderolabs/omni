<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SelectPortal, type SelectPortalProps, useForwardProps } from 'reka-ui'

const props = defineProps<SelectPortalProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <SelectPortal v-bind="forwarded">
    <slot></slot>
  </SelectPortal>
</template>
