<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import pluralize from 'pluralize'
import prettyBytes from 'pretty-bytes'
import { computed, ref, watch, watchEffect } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type {
  MachineInstallDiskConfigSpec,
  MachineInstallDiskStatusSpec,
  MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import { LabelControlPlaneRole, PatchBaseWeightClusterMachine } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TListItem from '@/components/List/TListItem.vue'
import ConfigPatchEditModal from '@/components/Modals/ConfigPatchEditModal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { getPatch } from '@/methods/getPatch'
import type { InstallDiskSelectItem } from '@/methods/installdisk'
import { installDiskFallbackItem, installDiskSelectItems } from '@/methods/installdisk'
import type { Label } from '@/methods/labels'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import { useMachineName } from '@/methods/node'
import type { MachineSetNode } from '@/states/cluster-management'
import { automaticInstallDisk, selectorInstallDisk, state } from '@/states/cluster-management'
import CreateExtensionsModal from '@/views/Clusters/components/CreateExtensionsModal.vue'
import MachineItemLabels from '@/views/ItemLabels/ItemLabels.vue'

import type { PickerOption } from './MachineSetPicker.vue'
import MachineSetPicker from './MachineSetPicker.vue'

defineEmits<{
  filterLabel: [Label]
}>()

const {
  item,
  installDiskStatus,
  installDiskConfig,
  versionContract,
  reset = undefined,
  searchQuery = undefined,
  versionMismatch,
  autoInstallNotice = null,
} = defineProps<{
  item: Resource<MachineStatusSpec>
  /**
   * List of machine disks
   */
  installDiskStatus?: Resource<MachineInstallDiskStatusSpec>
  /**
   * Current disk selection
   */
  installDiskConfig?: Resource<MachineInstallDiskConfigSpec>
  versionContract: Resource<VersionContractSpec>
  reset?: number
  searchQuery?: string
  versionMismatch: string | null
  autoInstallNotice?: string | null
}>()

const machineName = useMachineName(() => item)

const machineSetNode = ref<MachineSetNode>({
  patches: {},
})
const machineSetIndex = ref<number>()
const configPatchEditModalOpen = ref(false)
const createExtensionsModalOpen = ref(false)
const blockdevices = computed(() => item.spec.hardware?.blockdevices || [])
const systemDiskPath = computed(() => {
  const systemDisk = blockdevices.value.find((device) => device.system_disk)

  // dev_path with a linux_name fallback for machines polled before the dev_path field existed
  return systemDisk?.dev_path || systemDisk?.linux_name
})

const existingDisk = computed(() => installDiskConfig?.spec.disk)
const existingSelector = computed(() => installDiskConfig?.spec.disk_selector)

const disks = computed(() => {
  // the backend evaluates and orders the disks, the entries just render its verdicts
  const selectItems = installDiskSelectItems(installDiskStatus?.spec)
  // The two resources are independent, so the parenthesized disk is attached only when the
  // selection hash on the status agrees with the entry: empty hash for "Auto", non-empty for
  // "Selector".
  const selectionHash = installDiskStatus?.spec.selection_hash
  const resolvedInstallDisk = installDiskStatus?.spec.disk

  const automaticLabel =
    !installDiskConfig && resolvedInstallDisk && !selectionHash
      ? `${automaticInstallDisk} (${resolvedInstallDisk})`
      : automaticInstallDisk

  const items: InstallDiskSelectItem[] = [{ label: automaticLabel, value: automaticInstallDisk }]

  if (existingSelector.value) {
    items.push({
      label:
        resolvedInstallDisk && selectionHash
          ? `${selectorInstallDisk} (${resolvedInstallDisk})`
          : selectorInstallDisk,
      value: selectorInstallDisk,
      tooltip: existingSelector.value,
    })
  }

  const added = new Set<string>(selectItems.map((item) => item.value))

  // a disk referenced by the existing selection or the resolution but absent from the offered
  // entries gets a fallback entry (pickable or disabled, see installDiskFallbackItem)
  for (const disk of [existingDisk.value, resolvedInstallDisk]) {
    if (disk && !added.has(disk)) {
      added.add(disk)
      items.push(installDiskFallbackItem(disk, installDiskStatus?.spec))
    }
  }

  return [...items, ...selectItems]
})

// A pending pick whose entry disappears (e.g. the disk turns out to be an md array member on
// the next poll) is cleared, so a selection the dropdown no longer offers cannot be submitted.
// The status check skips the transient empty state of a reconnecting watch.
watchEffect(() => {
  const pendingSelection = machineSetNode.value.installDiskConfig

  if (typeof pendingSelection !== 'string' || !installDiskStatus) {
    return
  }

  const values = disks.value.filter((entry) => !entry.disabled).map((entry) => entry.value)

  if (!values.includes(pendingSelection)) {
    delete machineSetNode.value.installDiskConfig
  }
})

watch(
  state.value,
  () => {
    const index = state.value.machineSets.findIndex(
      (machineSet) => !!machineSet.machines[item.metadata.id!],
    )

    machineSetIndex.value = index === -1 ? undefined : index
  },
  { immediate: true },
)

watch(
  () => versionMismatch,
  (value) => {
    if (value) {
      machineSetIndex.value = undefined
    }
  },
)

watch(machineSetIndex, (val, old) => {
  if (val !== undefined) {
    state.value.setMachine(val, item.metadata.id!, machineSetNode.value)
  }

  if (old !== undefined) {
    state.value.removeMachine(old, item.metadata.id!)
  }
})

watch(
  () => reset,
  () => (machineSetIndex.value = undefined),
)

watch(machineSetNode, () => {
  if (!machineSetIndex.value) {
    return
  }

  state.value.setMachine(machineSetIndex.value, item.metadata.id!, machineSetNode.value)
})

const memoryModules = computed(() =>
  (item.spec.hardware?.memory_modules || []).filter((mem) => mem.size_mb),
)

const options = computed(() => {
  let memoryCapacity = 0
  for (const mem of memoryModules.value) {
    memoryCapacity += mem.size_mb ?? 0
  }

  const cpMemoryThreshold = 2 * 1024
  const workerMemoryTheshold = 1024

  const canUseAsControlPlane = memoryCapacity === 0 || memoryCapacity >= cpMemoryThreshold
  const canUseAsWorker = memoryCapacity === 0 || memoryCapacity >= workerMemoryTheshold

  return state.value.machineSets.map<PickerOption>((ms) => {
    const reasons: string[] = []
    let disabled = false

    if (ms.role === LabelControlPlaneRole && !canUseAsControlPlane) {
      disabled = true
      reasons.push(
        `The node must have more than ${prettyBytes(cpMemoryThreshold * 1024 * 1024, { binary: true })} of RAM to be used as a control plane`,
      )
    } else if (ms.role !== LabelControlPlaneRole && !canUseAsWorker) {
      disabled = true
      reasons.push(
        `The node must have more than ${prettyBytes(workerMemoryTheshold * 1024 * 1024, { binary: true })} of RAM to be used as a worker`,
      )
    }

    if (ms.machineAllocation) {
      disabled = true
      reasons.push(
        `The machine class ${ms.id} is using machine class so no manual allocation is possible`,
      )
    }

    if (versionMismatch) {
      disabled = true
      reasons.push(versionMismatch)
    }

    if (autoInstallNotice) {
      reasons.push(autoInstallNotice)
    }

    return {
      id: ms.id,
      disabled: disabled,
      tooltip: reasons.length > 0 ? reasons.join('\n\n') : undefined,
      labelClass: ms.labelClass,
    }
  })
})

const machinePatchID = computed(() => `cm-${item.metadata.id}`)

// The dropdown selection is a pending choice, recorded only when the user touches the
// dropdown and applied on submit (see reconcileInstallDiskConfigs). An untouched machine gets
// no writes at all. Picking a concrete disk always pins it, even when it equals the
// automatically resolved one.
const selectedInstallDisk = computed<string>({
  get() {
    const pendingSelection = machineSetNode.value.installDiskConfig

    if (pendingSelection === null) return automaticInstallDisk
    if (pendingSelection !== undefined) return pendingSelection
    if (existingDisk.value) return existingDisk.value
    if (existingSelector.value) return selectorInstallDisk

    return automaticInstallDisk
  },
  set(value) {
    switch (value) {
      // "Auto" always records the delete: the config watch may not have delivered yet, and
      // deleting a missing selection is tolerated on submit.
      case automaticInstallDisk:
        machineSetNode.value.installDiskConfig = null
        break
      case selectorInstallDisk:
      case existingDisk.value:
        delete machineSetNode.value.installDiskConfig
        break
      default:
        machineSetNode.value.installDiskConfig = value
    }
  },
})

const systemExtensions = ref<string[]>()

const exampleConfigPatch = computedAsync(async () => {
  const patch = await getPatch(versionContract.spec, 'hostname', {
    hostname: item.metadata.id ?? '',
  })

  return (
    machineSetNode.value.patches[machinePatchID.value]?.data ??
    `# Machine config patch for node "${item.metadata.id}"

# You can write partial Talos machine config here which will override the default
# Talos machine config for this machine generated by Omni.

# example (changing the node hostname):
${patch}
`
  )
})

const onSavePatchConfig = (config: string) => {
  if (!config) {
    delete machineSetNode.value.patches[machinePatchID.value]

    return
  }

  machineSetNode.value.patches[machinePatchID.value] = {
    data: config,
    weight: PatchBaseWeightClusterMachine,
  }
}
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center text-naturals-n13">
        <span class="grow truncate pr-2 font-bold">
          <WordHighlighter
            :query="searchQuery ?? ''"
            :text-to-highlight="machineName"
            split-by-space
            highlight-class="bg-naturals-n14"
          />
        </span>

        <template v-if="machineSetIndex !== undefined">
          <Tooltip :description="installDiskStatus?.spec.message" placement="bottom">
            <div
              v-if="systemDiskPath"
              class="cursor-not-allowed rounded border border-naturals-n6 py-1.5 pr-8 pl-3 text-naturals-n11"
            >
              Install Disk: {{ systemDiskPath }}
            </div>
            <div v-else>
              <TSelectList
                v-model="selectedInstallDisk"
                class="h-7"
                title="Install Disk"
                :values="disks"
              />
            </div>
          </Tooltip>
        </template>

        <MachineSetPicker v-model="machineSetIndex" :options="options" />

        <div class="flex items-center gap-1">
          <IconButton
            :id="
              machineSetIndex !== undefined
                ? `extensions-${options?.[machineSetIndex]?.id}`
                : undefined
            "
            class="my-auto text-naturals-n14"
            :disabled="machineSetIndex === undefined || options?.[machineSetIndex]?.disabled"
            :icon="systemExtensions ? 'extensions-toggle' : 'extensions'"
            @click="createExtensionsModalOpen = true"
          />
          <IconButton
            :id="machineSetIndex !== undefined ? options?.[machineSetIndex]?.id : undefined"
            class="my-auto text-naturals-n14"
            :disabled="machineSetIndex === undefined || options?.[machineSetIndex]?.disabled"
            :icon="
              machineSetNode.patches[machinePatchID] && machineSetIndex !== undefined
                ? 'settings-toggle'
                : 'settings'
            "
            @click="configPatchEditModalOpen = true"
          />
        </div>
      </div>

      <ConfigPatchEditModal
        :id="`Node ${item.metadata.id}`"
        v-model:open="configPatchEditModalOpen"
        :config="exampleConfigPatch"
        :talos-version="state.cluster.talosVersion"
        @save="onSavePatchConfig"
      />

      <CreateExtensionsModal
        v-model:open="createExtensionsModalOpen"
        :machine="item.metadata.id!"
        :model-value="systemExtensions"
        @save="
          (extensions) => {
            machineSetNode.systemExtensions = extensions
            systemExtensions = extensions
          }
        "
      />
    </template>

    <template #secondary>
      <MachineItemLabels
        :resource="item"
        :add-label-func="addMachineLabels"
        :remove-label-func="removeMachineLabels"
        @select-label="(label) => $emit('filterLabel', label)"
      />
    </template>

    <template #details>
      <div class="grid grid-cols-5 pl-6">
        <div class="mt-4 mb-2">Processors</div>
        <div class="mt-4 mb-2">Memory</div>
        <div class="mt-4 mb-2">Block Devices</div>
        <div class="mt-4 mb-2">Addresses</div>
        <div class="mt-4 mb-2">Network Interfaces</div>
        <div>
          <div v-for="(processor, index) in item?.spec?.hardware?.processors" :key="index">
            <template v-if="processor.frequency">{{ processor.frequency / 1000 }} GHz,</template>
            {{ processor.core_count }} {{ pluralize('core', processor.core_count) }},
            {{ processor.description }}
          </div>
        </div>
        <div>
          <div v-for="(mem, index) in memoryModules" :key="index">
            {{ prettyBytes((mem?.size_mb || 0) * 1024 * 1024, { binary: true }) }}
            {{ mem.description }}
          </div>
        </div>
        <div>
          <div v-for="(dev, index) in item?.spec?.hardware?.blockdevices" :key="index">
            {{ dev.linux_name }} {{ prettyBytes(Number(dev.size || '0')) }}
            {{ dev.type }}
          </div>
        </div>
        <div>
          <div>
            {{ item.spec?.network?.addresses?.join(', ') }}
          </div>
        </div>
        <div>
          <div v-for="(link, index) in item?.spec?.network?.network_links" :key="index">
            {{ link.linux_name }} {{ link.hardware_address }} {{ link.link_up ? 'UP' : 'DOWN' }}
          </div>
        </div>
      </div>
    </template>
  </TListItem>
</template>
