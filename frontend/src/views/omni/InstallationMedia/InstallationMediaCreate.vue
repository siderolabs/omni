<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'
import Stepper from '@/components/common/Stepper/Stepper.vue'
import EntryStep from '@/views/omni/InstallationMedia/Steps/Entry.vue'

const currentStep = ref(0)
const stepCount = ref(5)

const CurrentStepComponent = computed(() => {
  switch (currentStep.value) {
    case 0:
      return EntryStep
    default:
      return null
  }
})
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="p-6">
      <h1 class="mb-6 text-xl font-medium text-naturals-n14">Create New Media</h1>

      <component :is="CurrentStepComponent" />
    </div>

    <div class="shrink grow"></div>

    <div
      class="flex w-full items-center gap-4 border-t border-naturals-n4 bg-naturals-n1 px-4 max-md:flex-col max-md:p-4 md:h-16 md:justify-end"
    >
      <Stepper
        v-if="currentStep > 0"
        v-model="currentStep"
        :step-count="stepCount"
        class="mx-auto w-full"
      />

      <div class="flex gap-2 max-md:self-end">
        <TButton v-if="currentStep > 0" @click="currentStep--">Back</TButton>
        <TButton type="highlighted" @click="currentStep++">Next</TButton>
      </div>
    </div>
  </div>
</template>
