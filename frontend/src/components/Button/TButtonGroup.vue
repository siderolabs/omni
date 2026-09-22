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
    :class="cn('flex gap-0.5 rounded bg-naturals-n3 p-1', props.class)"
  >
    <RadioGroupItem
      v-for="(o, index) in options"
      :key="index"
      :value="o.value"
      :disabled="o.disabled"
      class="rounded border-naturals-n5 text-xs text-naturals-n10 transition-colors duration-200 hover:bg-naturals-n5 hover:text-naturals-n12 data-disabled:cursor-not-allowed data-disabled:text-naturals-n8 data-disabled:hover:bg-naturals-n3 data-[state=checked]:bg-naturals-n6 data-[state=checked]:text-primary-p3"
    >
      <Tooltip :description="o.tooltip" placement="top">
        <span class="inline-block px-2 py-0.5">{{ o.label || o.value }}</span>
      </Tooltip>
    </RadioGroupItem>
  </RadioGroupRoot>
</template>
