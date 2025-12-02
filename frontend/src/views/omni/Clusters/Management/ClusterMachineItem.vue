<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import yaml from 'js-yaml'
import pluralize from 'pluralize'
import semver from 'semver'
import { computed, ref, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { MachineConfigGenOptionsSpec, MachineStatusSpec } from '@/api/omni/specs/omni.pb'
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

defineEmits<{
  filterLabel: [Label]
}>()

const {
  item,
  reset = undefined,
  searchQuery = undefined,
  versionMismatch,
} = defineProps<{
  item: Resource<MachineStatusSpec & SiderolinkSpec & MachineConfigGenOptionsSpec>
  reset?: number
  searchQuery?: string
  versionMismatch: string | null
}>()

const machineSetNode = ref<MachineSetNode>({
  patches: {},
})
const machineSetIndex = ref<number>()
const blockdevices = computed(() => item.spec.hardware?.blockdevices || [])
const systemDiskPath = computed(
  () => blockdevices.value.find((device) => device.system_disk)?.linux_name,
)
const disks = computed(() =>
  blockdevices.value
    .filter((device) => !device.readonly && device.type !== 'CD')
    .map((device) => device.linux_name!),
)
const defaultInstallDisk = computed(() => item.spec.install_disk ?? disks.value[0])

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

const options = computed<PickerOption[]>(() => {
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

    if (versionMismatch) {
      disabled = true
      tooltip = versionMismatch
    }

    return {
      id: ms.id,
      disabled: disabled,
      tooltip: tooltip,
      color: ms.color,
    }
  })
})

const machinePatchID = computed(() => `cm-${item.metadata.id}`)
const installDiskPatchID = computed(() => `cm-${item.metadata.id}-${PatchID.InstallDisk}`)

const setInstallDisk = (value: string) => {
  if (value === defaultInstallDisk.value) {
    delete machineSetNode.value.patches[installDiskPatchID.value]
    return
  }

  machineSetNode.value.patches[installDiskPatchID.value] = {
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
    machine: item.metadata.id!,
    modelValue: systemExtensions.value,
    onSave(extensions?: string[]) {
      machineSetNode.value.systemExtensions = extensions

      systemExtensions.value = extensions
    },
  })
}

const openPatchConfig = () => {
  const isTalos112 = semver.satisfies(state.value.cluster.talosVersion!, '>=1.12', {
    includePrerelease: true,
  })

  const talos112 = `apiVersion: v1alpha1
kind: HostnameConfig
hostname: "${item.metadata.id}"
auto: off`

  const preTalos112 = `machine:
  network:
    hostname: "${item.metadata.id}"`

  showModal(ConfigPatchEdit, {
    tabs: [
      {
        config:
          machineSetNode.value.patches[machinePatchID.value]?.data ??
          `# Machine config patch for node "${item.metadata.id}"

# You can write partial Talos machine config here which will override the default
# Talos machine config for this machine generated by Omni.

# example (changing the node hostname):
${isTalos112 ? talos112 : preTalos112}
`,
        id: `Node ${item.metadata.id}`,
      },
    ],
    onSave(config: string) {
      if (!config) {
        delete machineSetNode.value.patches[machinePatchID.value]

        return
      }

      machineSetNode.value.patches[machinePatchID.value] = {
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
                :default-value="defaultInstallDisk"
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
