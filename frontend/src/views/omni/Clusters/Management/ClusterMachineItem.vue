<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import yaml from 'js-yaml'
import pluralize from 'pluralize'
import type { Ref } from 'vue'
import { computed, ref, toRefs, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type {
  MachineConfigGenOptionsSpec,
  MachineStatusSpec,
  MachineStatusSpecHardwareStatusBlockDevice,
} from '@/api/omni/specs/omni.pb'
import type { SiderolinkSpec } from '@/api/omni/specs/siderolink.pb'
import {
  LabelControlPlaneRole,
  PatchBaseWeightClusterMachine,
  PatchWeightInstallDisk,
} from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import { formatBytes } from '@/methods'
import type { Label } from '@/methods/labels'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import { showModal } from '@/modal'
import type { MachineSet, MachineSetNode } from '@/states/cluster-management'
import { PatchID, state } from '@/states/cluster-management'
import MachineItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import ConfigPatchEdit from '@/views/omni/Modals/ConfigPatchEdit.vue'

import CreateExtensions from '../../Modals/CreateExtensions.vue'
import type { PickerOption } from './MachineSetPicker.vue'
import MachineSetPicker from './MachineSetPicker.vue'

type MemModule = {
  size_mb?: number
  description?: string
}

defineEmits<{
  filterLabel: [Label]
}>()

const props = defineProps<{
  item: Resource<MachineStatusSpec & SiderolinkSpec & MachineConfigGenOptionsSpec>
  reset?: number
  searchQuery?: string
  versionMismatch: string | null
}>()

const { item, reset, versionMismatch } = toRefs(props)

const machineSetNode = ref<MachineSetNode>({
  patches: {},
})
const machineSetIndex = ref<number | undefined>()
const systemDiskPath: Ref<string | undefined> = ref()
const disks: Ref<string[]> = ref([])

const computeState = () => {
  const bds: MachineStatusSpecHardwareStatusBlockDevice[] =
    item?.value?.spec?.hardware?.blockdevices || []
  const diskPaths: string[] = []

  const index = bds?.findIndex(
    (device: MachineStatusSpecHardwareStatusBlockDevice) => device.system_disk,
  )
  if (index >= 0) {
    systemDiskPath.value = bds[index].linux_name
  }

  for (const device of bds) {
    if (device.readonly || device.type === 'CD') {
      continue
    }

    diskPaths.push(device.linux_name!)
  }

  disks.value = diskPaths

  computeMachineAssignment()
}

const computeMachineAssignment = () => {
  for (let i = 0; i < state.value.machineSets.length; i++) {
    const machineSet = state.value.machineSets[i]

    for (const id in machineSet.machines) {
      if (item.value.metadata.id === id) {
        machineSetIndex.value = i

        return
      }
    }
  }

  machineSetIndex.value = undefined
}

computeState()

watch(item, computeState)
watch(state.value, computeMachineAssignment)

watch(versionMismatch, (value: string | null) => {
  if (value) {
    machineSetIndex.value = undefined
  }
})

watch(machineSetIndex, (val?: number, old?: number) => {
  if (val !== undefined) {
    state.value.setMachine(val, item.value.metadata.id!, machineSetNode.value)
  }

  if (old !== undefined) {
    state.value.removeMachine(old, item.value.metadata.id!)
  }
})

if (reset.value) {
  watch(reset, () => {
    machineSetIndex.value = undefined
  })
}

watch(machineSetNode, () => {
  if (!machineSetIndex.value) {
    return
  }

  state.value.setMachine(machineSetIndex.value, item.value.metadata.id!, machineSetNode.value)
})

const filterValid = (modules: MemModule[]): MemModule[] => {
  return modules.filter((mem) => mem.size_mb)
}

const memoryModules = computed(() => {
  return filterValid(item?.value?.spec?.hardware?.memory_modules || [])
})

const options: Ref<PickerOption[]> = computed(() => {
  let memoryCapacity = 0
  for (const mem of memoryModules.value) {
    memoryCapacity += mem.size_mb ?? 0
  }

  const cpMemoryThreshold = 2 * 1024
  const workerMemoryTheshold = 1024

  const canUseAsControlPlane = memoryCapacity === 0 || memoryCapacity >= cpMemoryThreshold
  const canUseAsWorker = memoryCapacity === 0 || memoryCapacity >= workerMemoryTheshold

  return state.value.machineSets.map((ms: MachineSet) => {
    let disabled = ms.role === LabelControlPlaneRole ? !canUseAsControlPlane : !canUseAsWorker
    let tooltip: string | undefined

    if (disabled) {
      if (ms.role === LabelControlPlaneRole) {
        tooltip = `The node must have more than ${formatBytes(cpMemoryThreshold * 1024 * 1024)} of RAM to be used as a control plane`
      } else {
        tooltip = `The node must have more than ${formatBytes(workerMemoryTheshold * 1024 * 1024)} of RAM to be used as a worker`
      }
    }

    if (ms.machineAllocation) {
      disabled = true
      tooltip = `The machine class ${ms.id} is using machine class so no manual allocation is possible`
    }

    if (versionMismatch.value) {
      disabled = true
      tooltip = versionMismatch.value
    }

    return {
      id: ms.id,
      disabled: disabled,
      tooltip: tooltip,
      color: ms.color,
    }
  })
})

const machinePatchID = `cm-${item.value.metadata.id!}`
const installDiskPatchID = `cm-${item.value.metadata.id!}-${PatchID.InstallDisk}`

const setInstallDisk = (value: string) => {
  machineSetNode.value.patches[installDiskPatchID] = {
    data: yaml.dump({
      machine: {
        install: {
          disk: value,
        },
      },
    }),
    systemPatch: true,
    weight: PatchWeightInstallDisk,
    nameAnnotation: PatchID.InstallDisk,
  }
}

const systemExtensions = ref<string[]>()

const openExtensionConfig = () => {
  showModal(CreateExtensions, {
    machine: item.value.metadata.id!,
    modelValue: systemExtensions.value,
    onSave(extensions?: string[]) {
      machineSetNode.value.systemExtensions = extensions

      systemExtensions.value = extensions
    },
  })
}

const openPatchConfig = () => {
  showModal(ConfigPatchEdit, {
    tabs: [
      {
        config:
          machineSetNode.value.patches[machinePatchID]?.data ??
          `# Machine config patch for node "${item.value.metadata.id}"

# You can write partial Talos machine config here which will override the default
# Talos machine config for this machine generated by Omni.

# example (changing the node hostname):
machine:
  network:
    hostname: "${item.value.metadata.id}"
`,
        id: `Node ${item.value.metadata.id}`,
      },
    ],
    onSave(config: string) {
      if (!config) {
        delete machineSetNode.value.patches[machinePatchID]

        return
      }

      machineSetNode.value.patches[machinePatchID] = {
        data: config,
        weight: PatchBaseWeightClusterMachine,
      }
    },
  })
}
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center text-naturals-n13">
        <div class="flex flex-1 items-center gap-2 truncate">
          <span class="pr-2 font-bold">
            <WordHighlighter
              :query="searchQuery ?? ''"
              :text-to-highlight="item?.spec?.network?.hostname ?? item?.metadata?.id"
              split-by-space
              highlight-class="bg-naturals-n14"
            />
          </span>
          <MachineItemLabels
            :resource="item"
            :add-label-func="addMachineLabels"
            :remove-label-func="removeMachineLabels"
            @filter-label="(label) => $emit('filterLabel', label)"
          />
        </div>
        <div class="flex w-lg flex-initial items-center justify-end gap-4">
          <template v-if="machineSetIndex !== undefined">
            <div
              v-if="systemDiskPath"
              class="cursor-not-allowed rounded border border-naturals-n6 py-1.5 pr-8 pl-3 text-naturals-n11"
            >
              Install Disk: {{ systemDiskPath }}
            </div>
            <div v-else>
              <TSelectList
                class="h-7"
                title="Install Disk"
                :values="disks"
                :default-value="item.spec.install_disk ?? disks[0]"
                @checked-value="setInstallDisk"
              />
            </div>
          </template>
          <div>
            <MachineSetPicker
              :options="options"
              :machine-set-index="machineSetIndex"
              @update:machine-set-index="(value) => (machineSetIndex = value)"
            />
          </div>
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
              @click="openExtensionConfig"
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
              @click="openPatchConfig"
            />
          </div>
        </div>
      </div>
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
            {{ formatBytes((mem?.size_mb || 0) * 1024 * 1024) }} {{ mem.description }}
          </div>
        </div>
        <div>
          <div v-for="(dev, index) in item?.spec?.hardware?.blockdevices" :key="index">
            {{ dev.linux_name }} {{ formatBytes(dev.size) }} {{ dev.type }}
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
