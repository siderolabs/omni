<!--
Copyright (c) 2025 Sidero Labs, Inc.

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
import { computed } from 'vue'

interface Props extends StepperRootProps {
  stepCount: number
}

const { stepCount } = defineProps<Props>()
const model = defineModel<number>()

const steps = computed(() =>
  Array(stepCount)
    .fill(null)
    .map((_, i) => i + 1),
)
</script>

<template>
  <StepperRoot v-model="model" class="flex max-w-lg items-center">
    <StepperItem v-for="step in steps" :key="step" :step class="group contents">
      <StepperSeparator
        v-if="step !== steps[0]"
        class="mx-4 h-0.5 shrink grow rounded-full bg-primary-p4 group-data-[state=inactive]:bg-naturals-n6"
      />

      <StepperTrigger
        class="size-6 shrink-0 items-center justify-center rounded-sm border-2 border-primary-p4 bg-primary-p4 text-xs text-naturals-n14 shadow-sm group-data-disabled:cursor-not-allowed group-data-disabled:opacity-50 group-data-[state=inactive]:border-naturals-n6 group-data-[state=inactive]:bg-transparent group-data-[state=inactive]:text-naturals-n6"
      >
        <StepperIndicator>{{ step }}</StepperIndicator>
      </StepperTrigger>
    </StepperItem>
  </StepperRoot>
</template>
