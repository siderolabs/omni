<!--
Copyright (c) 2026 Sidero Labs, Inc.

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

import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'

const props = defineProps<
  DropdownMenuItemProps & {
    icon?: IconType
    danger?: boolean
  }
>()
const emit = defineEmits<DropdownMenuItemEmits>()

const emitsAsProps = useEmitAsProps(emit)
</script>

<template>
  <DropdownMenuItem
    class="flex w-full items-center gap-2 px-3 py-2 not-data-disabled:cursor-pointer data-disabled:cursor-not-allowed data-disabled:opacity-50"
    :class="
      danger
        ? 'text-red-r1 not-data-disabled:hover:text-primary-p1'
        : 'not-data-disabled:hover:text-naturals-n12'
    "
    v-bind="{ ...props, ...emitsAsProps }"
  >
    <TIcon v-if="icon" class="h-4 w-4 transition-colors" :icon />
    <span class="text-xs transition-colors">
      <slot></slot>
    </span>
  </DropdownMenuItem>
</template>
