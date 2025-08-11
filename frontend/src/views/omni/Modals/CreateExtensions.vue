<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, toRefs, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { closeModal } from '@/modal'
import ExtensionsPicker from '@/views/omni/Extensions/ExtensionsPicker.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const props = defineProps<{
  machine: string
  modelValue?: string[]
  onSave: (e?: string[]) => void
}>()

const { machine, modelValue } = toRefs(props)

const close = () => {
  closeModal()
}

const requestedExtensions = ref<Record<string, boolean>>({})

if (modelValue.value) {
  for (const key of modelValue.value) {
    requestedExtensions.value[key] = true
  }
}

const machineStatus = ref<Resource<MachineStatusSpec>>()
const machineStatusWatch = new Watch(machineStatus)

watch(machineStatus, () => {
  if (modelValue.value !== undefined) {
    return
  }

  const extensions = machineStatus.value?.spec.schematic?.extensions
  if (!extensions) {
    return
  }

  requestedExtensions.value = {}

  for (const extension of extensions) {
    requestedExtensions.value[extension] = true
  }
})

machineStatusWatch.setup(
  computed(() => {
    return {
      resource: {
        id: machine.value,
        namespace: DefaultNamespace,
        type: MachineStatusType,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const updateExtensions = (extensions?: Record<string, boolean>) => {
  if (extensions === undefined) {
    props.onSave()
  } else {
    const list: string[] = []
    for (const key in extensions) {
      if (!extensions[key]) {
        continue
      }

      list.push(key)
    }

    list.sort()

    props.onSave(list)
  }

  close()
}
</script>

<template>
  <div class="modal-window flex flex-col gap-4" style="height: 90%">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Set Extensions</h3>
      <CloseButton @click="close" />
    </div>

    <div v-if="machineStatus" class="flex flex-1 flex-col gap-4 overflow-hidden">
      <ExtensionsPicker
        v-model="requestedExtensions"
        :talos-version="machineStatus?.spec.talos_version!.slice(1)"
        class="flex-1"
      />

      <div class="flex justify-between gap-4">
        <TButton type="secondary" @click="close"> Cancel </TButton>
        <div class="flex gap-4">
          <TButton
            icon="reset"
            :disabled="modelValue === undefined"
            @click="() => updateExtensions()"
          >
            Revert
          </TButton>
          <TButton type="highlighted" @click="() => updateExtensions(requestedExtensions)">
            Save
          </TButton>
        </div>
      </div>
    </div>
    <div v-else class="flex items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-auto w-1/2 p-8;
}

.heading {
  @apply flex items-center justify-between text-xl text-naturals-n14;
}
</style>
