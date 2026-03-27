<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  StepperIndicator,
  StepperItem,
  StepperRoot,
  type StepperRootProps,
  StepperSeparator,
  StepperTrigger,
} from 'reka-ui'

interface Props extends Omit<StepperRootProps, 'modelValue'> {
  stepCount: number
}

const { stepCount, ...attrs } = defineProps<Props>()
const model = defineModel<number>()
</script>

<template>
  <StepperRoot v-model="model" v-bind="attrs" class="flex max-w-lg items-center">
    <StepperItem v-for="step in stepCount" :key="step" :step class="group contents">
      <StepperSeparator
        v-if="step !== 1"
        class="mx-4 h-0.5 shrink grow rounded-full bg-primary-p4 group-data-[state=inactive]:bg-naturals-n6"
      />

      <StepperTrigger
        class="size-6 shrink-0 items-center justify-center rounded-sm border-2 border-primary-p4 bg-primary-p4 text-xs text-naturals-n14 shadow-sm transition group-data-disabled:cursor-not-allowed group-data-disabled:opacity-50 group-data-[state=inactive]:border-naturals-n6 group-data-[state=inactive]:bg-transparent group-data-[state=inactive]:text-naturals-n6 not-group-data-disabled:hover:brightness-125 active:brightness-75"
      >
        <StepperIndicator>{{ step }}</StepperIndicator>
      </StepperTrigger>
    </StepperItem>
  </StepperRoot>
</template>
