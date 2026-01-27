<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import type { HardwareType } from '@/views/omni/InstallationMedia/useFormState'

const flows: Record<HardwareType, string[]> = {
  metal: [
    'InstallationMediaCreateTalosVersion',
    'InstallationMediaCreateMachineArch',
    'InstallationMediaCreateSystemExtensions',
    'InstallationMediaCreateExtraArgs',
    'InstallationMediaCreateConfirmation',
  ],
  cloud: [
    'InstallationMediaCreateTalosVersion',
    'InstallationMediaCreateCloudProvider',
    'InstallationMediaCreateMachineArch',
    'InstallationMediaCreateSystemExtensions',
    'InstallationMediaCreateExtraArgs',
    'InstallationMediaCreateConfirmation',
  ],
  sbc: [
    'InstallationMediaCreateTalosVersion',
    'InstallationMediaCreateSBCType',
    'InstallationMediaCreateSystemExtensions',
    'InstallationMediaCreateExtraArgs',
    'InstallationMediaCreateConfirmation',
  ],
}
</script>

<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { computed, ref, watch, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import Stepper from '@/components/common/Stepper/Stepper.vue'
import { showSuccess } from '@/notification'
import SavePresetModal from '@/views/omni/InstallationMedia/SavePresetModal.vue'
import { useFormState } from '@/views/omni/InstallationMedia/useFormState'

const router = useRouter()
const route = useRoute()

const { formState } = useFormState()
const isSaved = useSessionStorage('_installation_media_form_saved', false)

watch(
  formState,
  () => {
    // Clear saved state if we modify the form in any way
    isSaved.value = false
  },
  { deep: true },
)

watchEffect(() => {
  if (route.name === 'InstallationMediaCreateEntry' || formState.value.hardwareType) return

  // Fail-safe to return to the start of the form if we load a future step with a blank form state.
  if (window.history.length > 1) {
    // Using go(-1) to programmatically reverse backwards from history till we are out of this state.
    // This prevents having to use browser back multiple times and getting stuck in a history trap.
    router.go(-1)
    return
  }

  // In case of no history (e.g. opening a direct link), replace current route.
  router.replace({ name: 'InstallationMediaCreateEntry' })
})

const currentFlowSteps = computed(() =>
  formState.value.hardwareType ? flows[formState.value.hardwareType] : null,
)

const stepCount = computed(() => currentFlowSteps.value?.length ?? 0)
const currentStep = computed(() => {
  const currentStepName = route.name?.toString()

  return currentStepName && currentFlowSteps.value
    ? currentFlowSteps.value?.indexOf(currentStepName) + 1
    : 0
})

const isLastStep = computed(() =>
  currentFlowSteps.value ? currentStep.value === stepCount.value : false,
)

const nextStep = computed(() =>
  !isLastStep.value ? currentFlowSteps.value?.[currentStep.value] : undefined,
)

const prevStep = computed(() =>
  currentStep.value > 1
    ? currentFlowSteps.value?.[currentStep.value - 2]
    : 'InstallationMediaCreateEntry',
)

function onStepperChange(stepperValue?: number) {
  if (!currentFlowSteps.value || !stepperValue) return

  // Stepper is not 0 indexed
  router.push({ name: currentFlowSteps.value[stepperValue - 1] })
}

const savePresetModalOpen = ref(false)

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

      <RouterView v-model="formState" />
    </div>

    <div
      class="flex w-full shrink-0 items-center gap-4 border-t border-naturals-n4 bg-naturals-n1 px-4 max-md:flex-col max-md:p-4 md:h-16 md:justify-end"
    >
      <div v-if="currentFlowSteps && currentStep > 0" class="flex grow gap-4">
        <Tooltip description="Reset wizard">
          <button
            type="button"
            class="group isolate size-6 shrink-0 overflow-hidden rounded-sm border border-red-r1 p-0.5 text-red-r1 transition hover:bg-red-r1 hover:text-naturals-n1 active:brightness-75"
            @click="formState = {}"
          >
            <TIcon icon="close" class="size-full" aria-label="reset wizard" />
          </button>
        </Tooltip>

        <Stepper
          :linear="false"
          :model-value="currentStep"
          :step-count="stepCount"
          class="mx-auto grow"
          @update:model-value="onStepperChange"
        />
      </div>

      <div class="flex items-center gap-2 max-md:self-end">
        <TButton
          is="router-link"
          v-if="currentFlowSteps && currentStep > 0"
          :disabled="isSaved"
          :to="{ name: prevStep }"
        >
          Back
        </TButton>

        <TButton
          is="router-link"
          v-if="!isLastStep"
          type="highlighted"
          :disabled="!nextStep"
          :to="{ name: nextStep }"
        >
          Next
        </TButton>

        <TButton
          is="router-link"
          v-else-if="isSaved"
          type="highlighted"
          :to="{ name: 'InstallationMedia' }"
        >
          Finished
        </TButton>

        <TButton v-else type="highlighted" @click="savePresetModalOpen = true">Save</TButton>
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
