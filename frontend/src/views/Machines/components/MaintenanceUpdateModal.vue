<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { compare } from 'semver'
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import { majorMinorVersion } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { machineId } = defineProps<{
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const selectedVersion = ref('')

const { data: talosVersions, loading } = useResourceWatch<TalosVersionSpec>(() => ({
  skip: !open.value,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const { data: machine } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    type: MachineStatusType,
    namespace: DefaultNamespace,
    id: machineId,
  },
  runtime: Runtime.Omni,
}))

watchEffect(() => {
  if (open.value) selectedVersion.value = machine.value?.spec.talos_version?.slice(1) ?? ''
})

const updating = ref(false)

const upgradeVersions = computed(() => {
  return talosVersions.value
    .filter((v) => !v.spec.deprecated)
    .sort((a, b) => compare(b.metadata.id ?? '', a.metadata.id ?? ''))
    .reduce<Record<string, string[]>>((result, version) => {
      const versionId = version.metadata.id

      if (versionId) {
        const majorMinor = majorMinorVersion(versionId)

        result[majorMinor] ??= []
        result[majorMinor].push(versionId)
      }

      return result
    }, {})
})

const upgradeClick = async () => {
  if (machine.value?.spec.talos_version === `v${selectedVersion.value}`) {
    return
  }

  updating.value = true

  try {
    await ManagementService.MaintenanceUpgrade({
      machine_id: machineId,
      version: selectedVersion.value,
    })
  } catch (e) {
    showError('Failed to Do Maintenance Update', e.message)

    return
  } finally {
    updating.value = false

    open.value = false
  }

  showSuccess(
    'The Machine Update Triggered',
    `Machine ${machineId} is being updated to Talos version ${selectedVersion.value}`,
  )
}
</script>

<template>
  <Modal
    v-model:open="open"
    :title="`Update Talos on Node ${machineId}`"
    action-label="Update"
    :action-disabled="!talosVersions || updating"
    :loading="!talosVersions || updating"
    content-class="flex min-h-0 max-w-xl flex-1 flex-col gap-2"
    @confirm="upgradeClick"
  >
    <div class="shrink-0">
      <ManagedByTemplatesWarning warning-style="popup" />
    </div>

    <template v-if="!loading && machine">
      <RadioGroup
        v-model="selectedVersion"
        class="flex max-h-64 min-h-16 flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
      >
        <template v-for="(versions, group) in upgradeVersions" :key="group">
          <RadioGroupLabel as="div" class="w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold">
            {{ group }}
          </RadioGroupLabel>
          <div class="flex flex-col gap-1">
            <RadioGroupOption
              v-for="version in versions"
              :key="version"
              v-slot="{ checked }"
              :value="version"
            >
              <div
                class="tranform transition-color flex cursor-pointer items-center gap-2 px-2 py-1 text-sm hover:bg-naturals-n4"
                :class="{ 'bg-naturals-n4': checked }"
              >
                <TCheckbox
                  :model-value="checked"
                  class="pointer-events-none"
                  @vue:mounted="
                    ($event) =>
                      checked && ($event.el as HTMLElement).scrollIntoView({ block: 'center' })
                  "
                />
                {{ version }}
                <span v-if="version === machine.spec.talos_version?.slice(1)">(current)</span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>
  </Modal>
</template>
