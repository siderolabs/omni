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

import TButtonGroupItem from './primitives/TButtonGroupItem.vue'
import TButtonGroupRoot from './primitives/TButtonGroupRoot.vue'

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
  <TButtonGroupRoot :class="props.class">
    <RadioGroupRoot v-bind="forwarded">
      <TButtonGroupItem v-for="(o, index) in options" :key="index">
        <RadioGroupItem :value="o.value" :disabled="o.disabled">
          <Tooltip :description="o.tooltip" placement="top">
            <span class="inline-block px-tight py-micro">{{ o.label || o.value }}</span>
          </Tooltip>
        </RadioGroupItem>
      </TButtonGroupItem>
    </RadioGroupRoot>
  </TButtonGroupRoot>
</template>
