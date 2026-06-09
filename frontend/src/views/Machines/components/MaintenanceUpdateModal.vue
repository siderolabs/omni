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
import { majorMinorVersion } from '@/methods'
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

interface VersionGroup {
  versions: string[]
  unsupported: boolean
}

const upgradeVersions = computed(() => {
  if (!currentVersion.value) return {}

  const upgradeVersions =
    versionMap.value.get(currentVersion.value)?.spec.upgradable_talos_versions ?? []

  return [currentVersion.value, ...upgradeVersions]
    .map((v) => versionMap.value.get(v))
    .filter(
      (v): v is NonNullable<typeof v> =>
        !!v && (!v.spec.deprecated || v.spec.version === currentVersion.value),
    )
    .sort((a, b) => compare(b.spec.version!, a.spec.version!))
    .reduce<Record<string, VersionGroup>>((prev, { spec: { unsupported, version } }) => {
      const majorMinor = majorMinorVersion(version!)

      prev[majorMinor] ||= {
        versions: [],
        unsupported: true,
      }

      if (!unsupported || version === currentVersion.value) {
        prev[majorMinor].versions.push(version!)
        prev[majorMinor].unsupported = false
      }

      return prev
    }, {})
})

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
    content-class="flex max-w-xl flex-col gap-2"
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
        class="flex max-h-64 min-h-16 flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
      >
        <template v-for="(group, label) in upgradeVersions" :key="label">
          <RadioGroupLabel
            as="div"
            class="sticky top-0 w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold"
          >
            {{ `${label}${group.unsupported ? ' - Not supported by this Omni release' : ''}` }}
          </RadioGroupLabel>
          <div class="flex flex-col gap-1">
            <RadioGroupOption
              v-for="version in group.versions"
              :key="version"
              v-slot="{ checked }"
              :value="version"
            >
              <div
                class="flex transform cursor-pointer items-center gap-2 px-2 py-1 text-sm transition-colors hover:bg-naturals-n4"
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
                <span v-if="version === currentVersion">(current)</span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>
  </Modal>
</template>
