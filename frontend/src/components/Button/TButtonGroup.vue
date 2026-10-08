<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  type AcceptableValue,
  RadioGroupItem,
  RadioGroupRoot,
  type RadioGroupRootEmits,
  type RadioGroupRootProps,
  useForwardPropsEmits,
} from 'reka-ui'
import type { ClassValue } from 'vue'

import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { cn } from '@/methods/utils'

const props = defineProps<
  RadioGroupRootProps & {
    class?: ClassValue
    options: {
      label: string
      value: AcceptableValue
      disabled?: boolean
      tooltip?: string
    }[]
  }
>()

const emit = defineEmits<RadioGroupRootEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'options')
const forwarded = useForwardPropsEmits(delegatedProps, emit)
</script>

<template>
  <RadioGroupRoot
    v-bind="forwarded"
    :class="cn('flex gap-0.5 rounded bg-surface-raised p-micro', props.class)"
  >
    <RadioGroupItem
      v-for="(o, index) in options"
      :key="index"
      :value="o.value"
      :disabled="o.disabled"
      class="rounded border-border-strong text-xs text-content-muted transition-colors duration-200 hover:bg-surface-inert hover:text-content-default data-disabled:cursor-not-allowed data-disabled:text-content-muted data-disabled:hover:bg-surface-raised data-[state=checked]:bg-surface-inert data-[state=checked]:text-content-emphasis data-[state=checked]:ring-1 data-[state=checked]:ring-border-strong"
    >
      <Tooltip :description="o.tooltip" placement="top">
        <span class="inline-block px-tight py-0.5">{{ o.label || o.value }}</span>
      </Tooltip>
    </RadioGroupItem>
  </RadioGroupRoot>
</template>
