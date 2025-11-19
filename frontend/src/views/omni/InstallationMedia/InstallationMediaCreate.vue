<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
const flows = {
  metal: [
    TalosVersionStep,
    MachineArchStep,
    SystemExtensionsStep,
    ExternalArgsStep,
    ConfirmationStep,
  ],
  cloud: [
    TalosVersionStep,
    CloudProviderStep,
    MachineArchStep,
    SystemExtensionsStep,
    ExternalArgsStep,
    ConfirmationStep,
  ],
  sbc: [TalosVersionStep, SBCTypeStep, SystemExtensionsStep, ExternalArgsStep, ConfirmationStep],
} satisfies Record<string, Component[]>

type FlowType = keyof typeof flows

export interface FormState {
  currentStep: number
  name?: string
  hardwareType?: FlowType
  talosVersion?: string
  joinToken?: string
  machineArch?: 'amd64' | 'arm64'
  secureBoot?: boolean
  cloudPlatform?: string
  sbcType?: string
  systemExtensions?: string[]
}
</script>

<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { type Component, computed } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'
import Stepper from '@/components/common/Stepper/Stepper.vue'
import CloudProviderStep from '@/views/omni/InstallationMedia/Steps/CloudProvider.vue'
import ConfirmationStep from '@/views/omni/InstallationMedia/Steps/Confirmation.vue'
import EntryStep from '@/views/omni/InstallationMedia/Steps/Entry.vue'
import ExternalArgsStep from '@/views/omni/InstallationMedia/Steps/ExternalArgs.vue'
import MachineArchStep from '@/views/omni/InstallationMedia/Steps/MachineArch.vue'
import SBCTypeStep from '@/views/omni/InstallationMedia/Steps/SBCType.vue'
import SystemExtensionsStep from '@/views/omni/InstallationMedia/Steps/SystemExtensions.vue'
import TalosVersionStep from '@/views/omni/InstallationMedia/Steps/TalosVersion.vue'

const formState = useSessionStorage<FormState>(
  '_installation_media_form',
  { currentStep: 0 },
  { writeDefaults: false },
)

const currentFlowSteps = computed(() =>
  formState.value.hardwareType ? flows[formState.value.hardwareType] : null,
)
const currentStepComponent = computed(() =>
  currentFlowSteps.value && formState.value.currentStep
    ? currentFlowSteps.value[formState.value.currentStep - 1]
    : EntryStep,
)

const stepCount = computed(() => currentFlowSteps.value?.length ?? 0)
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex grow flex-col gap-6 overflow-auto p-6">
      <h1 class="shrink-0 text-xl font-medium text-naturals-n14">Create New Media</h1>

      <component :is="currentStepComponent" v-model="formState" />
    </div>

    <div
      class="flex w-full shrink-0 items-center gap-4 border-t border-naturals-n4 bg-naturals-n1 px-4 max-md:flex-col max-md:p-4 md:h-16 md:justify-end"
    >
      <Stepper
        v-if="currentFlowSteps && formState.currentStep > 0"
        v-model="formState.currentStep"
        :step-count="stepCount"
        class="mx-auto w-full"
      />

      <div class="flex gap-2 max-md:self-end">
        <TButton
          v-if="currentFlowSteps && formState.currentStep > 0"
          :disabled="formState.currentStep <= 0"
          @click="formState.currentStep = Math.max(0, formState.currentStep - 1)"
        >
          Back
        </TButton>

        <TButton
          type="highlighted"
          :disabled="formState.currentStep >= stepCount"
          @click="formState.currentStep = Math.min(stepCount, formState.currentStep + 1)"
        >
          Next
        </TButton>
      </div>
    </div>
  </div>
</template>
