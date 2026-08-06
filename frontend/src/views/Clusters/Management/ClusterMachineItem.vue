<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import pluralize from 'pluralize'
import prettyBytes from 'pretty-bytes'
import { computed, ref, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { MachineInstallDiskStatusSpec, MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb.ts'
import { LabelControlPlaneRole, PatchBaseWeightClusterMachine } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TListItem from '@/components/List/TListItem.vue'
import ConfigPatchEditModal from '@/components/Modals/ConfigPatchEditModal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { getPatch } from '@/methods/getPatch.ts'
import type { Label } from '@/methods/labels'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import { useMachineName } from '@/methods/node'
import type { MachineSetNode } from '@/states/cluster-management'
import { automaticInstallDisk, state } from '@/states/cluster-management'
import CreateExtensionsModal from '@/views/Clusters/components/CreateExtensionsModal.vue'
import MachineItemLabels from '@/views/ItemLabels/ItemLabels.vue'

import type { PickerOption } from './MachineSetPicker.vue'
import MachineSetPicker from './MachineSetPicker.vue'

defineEmits<{
  filterLabel: [Label]
}>()

const {
  item,
  versionContract,
  reset = undefined,
  searchQuery = undefined,
  versionMismatch,
  autoInstallNotice = null,
} = defineProps<{
  item: Resource<MachineStatusSpec & MachineInstallDiskStatusSpec>
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

// The backend resolves the install disk and the eligible candidates centrally
// (MachineInstallDiskStatus), the dropdown just renders them. The resolved disk may sit outside
// the candidates (an explicit selector can target any disk), so it is included explicitly.
const resolvedInstallDisk = computed(() => item.spec.disk)
const disks = computed(() => {
  const candidates = item.spec.candidates ?? []

  if (resolvedInstallDisk.value && !candidates.includes(resolvedInstallDisk.value)) {
    return [automaticInstallDisk, resolvedInstallDisk.value, ...candidates]
  }

  return [automaticInstallDisk, ...candidates]
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

// The selection is an intent recorded only when the user touches the dropdown: a dev path means
// create-or-update the MachineInstallDiskConfig, "automatic" means delete it, and re-picking the
// currently resolved disk clears the intent so an untouched machine gets no writes at all.
const setInstallDisk = (value: string) => {
  if (value === automaticInstallDisk) {
    machineSetNode.value.installDiskConfig = null
    return
  }

  if (value === resolvedInstallDisk.value) {
    delete machineSetNode.value.installDiskConfig
    return
  }

  machineSetNode.value.installDiskConfig = value
}

const shownInstallDisk = computed(() => {
  const intent = machineSetNode.value.installDiskConfig

  if (intent === null) {
    return automaticInstallDisk
  }

  return intent ?? resolvedInstallDisk.value ?? automaticInstallDisk
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
          <span
            v-if="item.spec.message"
            :title="item.spec.message"
            class="max-w-48 truncate text-xs text-naturals-n9"
          >
            {{ item.spec.message }}
          </span>
          <div
            v-if="systemDiskPath"
            class="cursor-not-allowed rounded border border-naturals-n6 py-1.5 pr-8 pl-3 text-naturals-n11"
          >
            Install Disk: {{ systemDiskPath }}
          </div>
          <div v-else class="flex items-center gap-2">
            <TSelectList
              class="h-7"
              title="Install Disk"
              :values="disks"
              :default-value="shownInstallDisk"
              @checked-value="setInstallDisk"
            />
          </div>
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
            {{ (processor.frequency ?? 0) / 1000 }} GHz, {{ processor.core_count }}
            {{ pluralize('core', processor.core_count) }}, {{ processor.description }}
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
