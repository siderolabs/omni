<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import Modal from '@/components/Modals/Modal.vue'
import { getMachineName } from '@/methods/node'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ExtensionsPicker from '@/views/Extensions/ExtensionsPicker.vue'

const { machine, modelValue } = defineProps<{
  machine: string
  modelValue?: string[]
}>()

const open = defineModel<boolean>('open', { default: false })

const emit = defineEmits<{
  save: [extensions?: string[]]
}>()

const selectedExtensionMap = ref<Record<string, boolean>>({})
const selectedExtensions = computed(() =>
  Object.keys(selectedExtensionMap.value)
    .filter((key) => !!selectedExtensionMap.value[key])
    .sort(),
)

const { data: machineStatus } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    id: machine,
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  runtime: Runtime.Omni,
}))

watchEffect(() => {
  const extensions = modelValue ?? machineStatus.value?.spec.schematic?.extensions

  if (extensions) {
    selectedExtensionMap.value = Object.fromEntries(extensions.map((e) => [e, true]))
  }
})

const updateExtensions = () => {
  emit('save', selectedExtensions.value.length ? selectedExtensions.value : undefined)

  open.value = false
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Select Extensions"
    :loading="!machineStatus?.spec.talos_version"
    action-label="Save"
    class="w-screen max-w-3xl"
    content-class="flex flex-col gap-2 overflow-hidden"
    @confirm="updateExtensions"
  >
    <template #description>
      Select the extensions to be installed on node
      {{ getMachineName(machineStatus) }}.
    </template>

    <TButton variant="subtle" icon="reset" class="self-end" @click="selectedExtensionMap = {}">
      Clear selection
    </TButton>

    <ExtensionsPicker
      v-if="machineStatus?.spec.talos_version"
      v-model="selectedExtensionMap"
      :talos-version="machineStatus.spec.talos_version.slice(1)"
      class="h-120"
    />
  </Modal>
</template>
