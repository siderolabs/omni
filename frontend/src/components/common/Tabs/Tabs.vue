<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  TabsIndicator,
  TabsList,
  TabsRoot,
  type TabsRootEmits,
  type TabsRootProps,
  useForwardPropsEmits,
} from 'reka-ui'

const props = defineProps<TabsRootProps>()
const emits = defineEmits<TabsRootEmits>()
const forwarded = useForwardPropsEmits(props, emits)

defineSlots<{
  triggers(): unknown
  contents(): unknown
}>()
</script>

<template>
  <TabsRoot class="flex flex-col" v-bind="forwarded">
    <TabsList class="relative flex shrink-0 gap-6 border-b border-naturals-n4 pb-3.5">
      <TabsIndicator
        class="absolute bottom-0 left-0 h-0.5 w-(--reka-tabs-indicator-size) translate-x-(--reka-tabs-indicator-position) translate-y-px transition-[width,translate] duration-300"
      >
        <div class="size-full bg-primary-p3"></div>
      </TabsIndicator>

      <slot name="triggers"></slot>
    </TabsList>

    <slot name="contents"></slot>
  </TabsRoot>
</template>
