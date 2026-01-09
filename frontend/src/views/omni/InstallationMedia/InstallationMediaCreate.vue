<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import { type Component } from 'vue'

import type { SchematicBootloader } from '@/api/omni/management/management.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TIcon from '@/components/common/Icon/TIcon.vue'
import type { LabelSelectItem } from '@/components/common/Labels/Labels.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

type HardwareType = 'metal' | 'cloud' | 'sbc'

const flows: Record<HardwareType, Component[]> = {
  metal: [TalosVersionStep, MachineArchStep, SystemExtensionsStep, ExtraArgsStep, ConfirmationStep],
  cloud: [
    TalosVersionStep,
    CloudProviderStep,
    MachineArchStep,
    SystemExtensionsStep,
    ExtraArgsStep,
    ConfirmationStep,
  ],
  sbc: [TalosVersionStep, SBCTypeStep, SystemExtensionsStep, ExtraArgsStep, ConfirmationStep],
}

export interface FormState {
  currentStep: number
  hardwareType?: HardwareType
  talosVersion?: string
  useGrpcTunnel?: boolean
  joinToken?: string
  machineUserLabels?: Record<string, LabelSelectItem>
  machineArch?: PlatformConfigSpecArch
  secureBoot?: boolean
  cloudPlatform?: string
  sbcType?: string
  systemExtensions?: string[]
  cmdline?: string
  overlayOptions?: string
  bootloader?: SchematicBootloader
}
</script>

<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import Stepper from '@/components/common/Stepper/Stepper.vue'
import { showSuccess } from '@/notification'
import SavePresetModal from '@/views/omni/InstallationMedia/SavePresetModal.vue'
import CloudProviderStep from '@/views/omni/InstallationMedia/Steps/CloudProvider.vue'
import ConfirmationStep from '@/views/omni/InstallationMedia/Steps/Confirmation.vue'
import EntryStep from '@/views/omni/InstallationMedia/Steps/Entry.vue'
import ExtraArgsStep from '@/views/omni/InstallationMedia/Steps/ExtraArgs.vue'
import MachineArchStep from '@/views/omni/InstallationMedia/Steps/MachineArch.vue'
import SBCTypeStep from '@/views/omni/InstallationMedia/Steps/SBCType.vue'
import SystemExtensionsStep from '@/views/omni/InstallationMedia/Steps/SystemExtensions.vue'
import TalosVersionStep from '@/views/omni/InstallationMedia/Steps/TalosVersion.vue'

const router = useRouter()
const defaultState: FormState = { currentStep: 0 }

const formState = useSessionStorage<FormState>('_installation_media_form', defaultState, {
  writeDefaults: false,
})

const currentFlowSteps = computed(() =>
  formState.value.hardwareType ? flows[formState.value.hardwareType] : null,
)
const currentStepComponent = computed(() =>
  currentFlowSteps.value && formState.value.currentStep
    ? currentFlowSteps.value[formState.value.currentStep - 1]
    : EntryStep,
)

const stepCount = computed(() => currentFlowSteps.value?.length ?? 0)
const isLastStep = computed(
  () => currentFlowSteps.value && formState.value.currentStep === stepCount.value,
)

function goBackStep() {
  formState.value.currentStep = Math.max(0, formState.value.currentStep - 1)
}

function goNextStep() {
  formState.value.currentStep = Math.min(stepCount.value, formState.value.currentStep + 1)
}

const savePresetModalOpen = ref(false)
const isSaved = ref(false)

function openSavePresetModal() {
  savePresetModalOpen.value = true
}

function goToPresetList() {
  formState.value = defaultState
  router.push({ name: 'InstallationMedia' })
}

function onSaved(name: string) {
  showSuccess(`Preset saved as ${name}`)

  isSaved.value = true
  savePresetModalOpen.value = false
}
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
      <div v-if="currentFlowSteps && formState.currentStep > 0" class="flex grow gap-4">
        <Tooltip description="Reset wizard">
          <button
            class="group isolate size-6 shrink-0 overflow-hidden rounded-sm border border-red-r1 p-0.5 text-red-r1 transition hover:bg-red-r1 hover:text-naturals-n1 active:brightness-75"
            aria-label="reset wizard"
            @click="formState = defaultState"
          >
            <TIcon icon="close" class="size-full" />
          </button>
        </Tooltip>

        <Stepper v-model="formState.currentStep" :step-count="stepCount" class="mx-auto grow" />
      </div>

      <div class="flex items-center gap-2 max-md:self-end">
        <TButton
          v-if="currentFlowSteps && formState.currentStep > 0"
          :disabled="formState.currentStep <= 0 || isSaved"
          @click="goBackStep"
        >
          Back
        </TButton>

        <TButton
          type="highlighted"
          @click="isLastStep ? (isSaved ? goToPresetList() : openSavePresetModal()) : goNextStep()"
        >
          {{ isLastStep ? (isSaved ? 'Finished' : 'Save') : 'Next' }}
        </TButton>
      </div>
    </div>

    <SavePresetModal
      :open="savePresetModalOpen"
      :form-state
      @close="savePresetModalOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
