<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui'
import { computed } from 'vue'

const {
  // eslint-disable-next-line vue/no-boolean-default
  open = undefined,
  keepOpen,
  placement = 'auto-start',
  delayDuration = 0,
  offsetDistance = 10,
  offsetSkid = 30,
} = defineProps<{
  open?: boolean
  keepOpen?: boolean
  delayDuration?: number
  disabled?: boolean
  description?: string
  offsetDistance?: number
  offsetSkid?: number
  placement?:
    | 'auto'
    | 'auto-start'
    | 'auto-end'
    | 'top'
    | 'top-start'
    | 'top-end'
    | 'bottom'
    | 'bottom-start'
    | 'bottom-end'
    | 'right'
    | 'right-start'
    | 'right-end'
    | 'left'
    | 'left-start'
    | 'left-end'
}>()

defineOptions({ inheritAttrs: false })

const side = computed(() => {
  const [side] = placement.split('-')
  return side === 'auto' ? undefined : (side as 'top' | 'bottom' | 'right' | 'left')
})

const align = computed(() => {
  const [, align = 'center'] = placement.split('-')
  return align as 'center' | 'start' | 'end'
})
</script>

<template>
  <TooltipProvider>
    <TooltipRoot
      :disable-hoverable-content="!keepOpen"
      :delay-duration
      :open
      :disabled="disabled || (!description && !$slots.description)"
    >
      <TooltipTrigger as-child>
        <slot></slot>
      </TooltipTrigger>

      <TooltipPortal>
        <TooltipContent
          class="z-50 rounded border border-naturals-n4 bg-naturals-n3 p-4 text-xs text-naturals-n12"
          :side-offset="offsetDistance"
          :align-offset="offsetSkid"
          :align
          :side
        >
          <slot name="description">
            <p class="whitespace-pre">{{ description }}</p>
          </slot>
        </TooltipContent>
      </TooltipPortal>
    </TooltipRoot>
  </TooltipProvider>
</template>
