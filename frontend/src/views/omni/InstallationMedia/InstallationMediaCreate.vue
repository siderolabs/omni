<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { SchematicBootloader } from '@/api/omni/management/management.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TIcon from '@/components/common/Icon/TIcon.vue'
import type { LabelSelectItem } from '@/components/common/Labels/Labels.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

type HardwareType = 'metal' | 'cloud' | 'sbc'

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

export interface FormState {
  isSaved?: boolean
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
import { computed, ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import Stepper from '@/components/common/Stepper/Stepper.vue'
import { showSuccess } from '@/notification'
import SavePresetModal from '@/views/omni/InstallationMedia/SavePresetModal.vue'

const router = useRouter()
const route = useRoute()

const formState = useSessionStorage<FormState>(
  '_installation_media_form',
  {},
  { writeDefaults: false },
)

watchEffect(() => {
  if (route.name === 'InstallationMediaCreateEntry') {
    // Reset form when on entry page
    formState.value = {}
  } else if (!formState.value.hardwareType) {
    // Fail-safe to return to the start of the form if we load a future step with a blank form state
    router.replace({ name: 'InstallationMediaCreateEntry' })
  }
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

  formState.value.isSaved = true
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
          <RouterLink
            class="group isolate size-6 shrink-0 overflow-hidden rounded-sm border border-red-r1 p-0.5 text-red-r1 transition hover:bg-red-r1 hover:text-naturals-n1 active:brightness-75"
            :to="{ name: 'InstallationMediaCreateEntry' }"
          >
            <TIcon icon="close" class="size-full" aria-label="reset wizard" />
          </RouterLink>
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
          :disabled="formState.isSaved"
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
          v-else-if="formState.isSaved"
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
