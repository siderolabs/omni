<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  DropdownMenuItem,
  type DropdownMenuItemEmits,
  type DropdownMenuItemProps,
  useEmitAsProps,
} from 'reka-ui'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

const props = defineProps<
  DropdownMenuItemProps & {
    icon?: IconType
    danger?: boolean
  }
>()
const emits = defineEmits<DropdownMenuItemEmits>()

const emitsAsProps = useEmitAsProps(emits)
</script>

<template>
  <DropdownMenuItem
    class="flex w-full cursor-pointer items-center gap-2 px-3 py-2"
    :class="danger ? 'text-red-r1 hover:text-primary-p1' : 'hover:text-naturals-n12'"
    v-bind="{ ...props, ...emitsAsProps }"
  >
    <TIcon v-if="icon" class="h-4 w-4 transition-colors" :icon />
    <span class="text-xs transition-colors">
      <slot></slot>
    </span>
  </DropdownMenuItem>
</template>
