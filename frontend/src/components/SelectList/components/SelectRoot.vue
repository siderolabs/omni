<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends AcceptableValue = AcceptableValue">
import {
  type AcceptableValue,
  SelectRoot,
  type SelectRootEmits,
  type SelectRootProps,
  useForwardPropsEmits,
} from 'reka-ui'

const props = defineProps<SelectRootProps<T>>()
const emit = defineEmits<SelectRootEmits<T>>()

const forwarded = useForwardPropsEmits(props, emit)
</script>

<template>
  <SelectRoot v-bind="forwarded">
    <slot></slot>
  </SelectRoot>
</template>
