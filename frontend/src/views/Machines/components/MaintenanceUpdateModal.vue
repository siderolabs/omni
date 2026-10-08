<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  MachineStatusLinkType,
  MetricsNamespace,
  TalosVersionType,
} from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import TAlert from '@/components/TAlert.vue'
import { talosUpgradeTargets } from '@/methods/talosUpgradeTargets'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { machineId } = defineProps<{
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const selectedVersion = ref('')
const updating = ref(false)

const { data: talosVersions, loading: talosVersionsLoading } = useResourceWatch<TalosVersionSpec>(
  () => ({
    skip: !open.value,
    resource: {
      type: TalosVersionType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
  }),
)

const { data: machine } = useResourceWatch<MachineStatusLinkSpec>(() => ({
  skip: !open.value,
  resource: {
    type: MachineStatusLinkType,
    namespace: MetricsNamespace,
    id: machineId,
  },
  runtime: Runtime.Omni,
}))

const versionMap = computed(() => new Map(talosVersions.value.map((v) => [v.metadata.id!, v])))
const currentVersion = computed(() => machine.value?.spec.message_status?.talos_version?.slice(1))

watchEffect(() => {
  if (open.value) selectedVersion.value = currentVersion.value ?? ''
})

const { installing, upgrading } = useDerivedMachineStage(() => machine.value?.spec.snapshot)

const inProgress = computed(() => installing.value || upgrading.value)
const inProgressMessage = computed(() =>
  installing.value
    ? 'A Talos install is already in progress on this machine.'
    : 'A Talos upgrade is already in progress on this machine.',
)

const upgradeVersions = computed(() => talosUpgradeTargets(versionMap.value, currentVersion.value))

const upgradeClick = async () => {
  if (machine.value?.spec.message_status?.talos_version === `v${selectedVersion.value}`) {
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
    title="Update Talos"
    action-label="Update"
    cancel-label="Close"
    :action-disabled="!talosVersions || updating || inProgress"
    :loading="talosVersionsLoading || updating"
    content-class="flex max-w-xl flex-col gap-tight"
    @confirm="upgradeClick"
  >
    <template #description>Node {{ machineId }}</template>

    <div class="shrink-0">
      <ManagedByTemplatesWarning warning-style="popup" />
    </div>

    <TAlert v-if="inProgress" type="warn" title="Upgrade in progress">
      {{ inProgressMessage }}
    </TAlert>

    <template v-if="!talosVersionsLoading && machine">
      <span v-if="!Object.keys(upgradeVersions).length">No versions found</span>

      <RadioGroup
        v-model="selectedVersion"
        class="flex max-h-64 min-h-16 flex-1 flex-col gap-tight overflow-y-auto text-content-default"
      >
        <template v-for="(group, label) in upgradeVersions" :key="label">
          <RadioGroupLabel
            as="div"
            class="sticky top-0 w-full bg-surface-hover p-micro pl-7 text-sm font-bold"
          >
            {{ `${label}${group.unsupported ? ' - Not supported by this Omni release' : ''}` }}
          </RadioGroupLabel>
          <div class="flex flex-col gap-micro">
            <RadioGroupOption
              v-for="version in group.versions"
              :key="version"
              v-slot="{ checked }"
              :value="version"
            >
              <div
                class="flex transform cursor-pointer items-center gap-tight px-tight py-micro text-sm transition-colors hover:bg-surface-hover"
                :class="{ 'bg-surface-hover': checked }"
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
                <span v-if="version === currentVersion">(current)</span>
                <div class="grow"></div>
                <span v-if="versionMap.get(version)?.spec.is_enterprise" class="resource-label">
                  enterprise
                </span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>
  </Modal>
</template>
