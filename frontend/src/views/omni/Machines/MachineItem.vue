<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { DateTime } from 'luxon'
import pluralize from 'pluralize'
import { computed, h } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'
import { copyText } from 'vue3-clipboard'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusSpecPowerState } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelInstalled } from '@/api/resources'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import IconButton from '@/components/common/Button/IconButton.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { formatBytes } from '@/methods'
import {
  canAccessMaintenanceNodes,
  canReadClusters,
  canReadMachineLogs,
  canRemoveMachines,
} from '@/methods/auth'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import MachineItemInfoCard from '@/views/omni/Machines/MachineItemInfoCard.vue'

const { machine, searchQuery = '' } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
  searchQuery?: string
}>()

defineEmits(['filterLabels'])

const router = useRouter()

const machineName = computed(() => {
  return machine.spec.message_status?.network?.hostname ?? machine.metadata.id
})

const openConfigPatches = () => {
  router.push({ name: 'MachineConfigPatches', params: { machine: machine.metadata.id } })
}

const openMaintenanceUpdate = () => {
  router.push({
    query: {
      modal: 'maintenanceUpdate',
      machine: machine.metadata.id,
      cluster: clusterName.value,
    },
  })
}

const processors = computed(() => {
  const processors = machine.spec.message_status?.hardware?.processors || []

  const format = (proc: { frequency?: number; core_count?: number; description?: string }) => {
    return `${proc.frequency! / 1000} GHz, ${pluralize('core', proc.core_count, true)}, ${proc.description}`
  }

  return processors.map(format)
})

const memorymodules = computed(() => {
  const memorymodules = machine.spec.message_status?.hardware?.memory_modules || []

  const format = (mem: { description?: string; size_mb?: number }) => {
    let description = mem.description

    if (mem.description === undefined) {
      description = ''
    }
    return `${formatBytes(mem.size_mb! * 1024 * 1024)} ${description}`
  }

  return memorymodules.filter((mem: { size_mb?: number }) => mem.size_mb !== 0).map(format)
})

const clusterName = computed(() => machine.spec.message_status?.cluster)

const removeMachine = () => {
  router.push({
    query: {
      modal: 'machineRemove',
      machine: machine.metadata.id,
      cluster: clusterName.value,
    },
  })
}

const machineLogs = () => {
  router.push({
    name: 'MachineLogs',
    params: { machine: machine.metadata.id },
  })
}

const showCluster = () => {
  router.push({
    name: 'ClusterOverview',
    params: { cluster: clusterName.value },
  })
}

const timeGetter = (fn: () => string | undefined) => {
  return computed(() => {
    const time = fn()
    if (time) {
      const dt: DateTime = /^\d+$/.exec(time)
        ? DateTime.fromSeconds(parseInt(time))
        : DateTime.fromISO(time)

      return dt.setLocale('en').toLocaleString(DateTime.DATETIME_FULL_WITH_SECONDS)
    }

    return 'Never'
  })
}

const machineLastAlive = timeGetter(() => machine.spec.siderolink_counter?.last_alive)
const machineCreatedAt = timeGetter(() => machine.spec.machine_created_at)
const secureBoot = computed(() => {
  const securityState = machine.spec.message_status?.security_state
  if (!securityState) {
    return 'Unknown'
  }

  return securityState.secure_boot ? 'Enabled' : 'Disabled'
})

const copyMachineID = () => {
  copyText(machine.metadata.id!, undefined, () => {})
}

const canDoMaintenanceUpdate = computed(() => {
  if (machine.spec.message_status?.cluster) {
    return false
  }

  return machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined
})

const maintenanceUpdateDescription = computed(() => {
  if (machine.spec.message_status?.cluster) {
    return 'Maintenance upgrade is not possible: machine is a part of a cluster'
  }

  if (!canDoMaintenanceUpdate.value) {
    return 'Maintenance upgrade is not possible: Talos is not installed'
  }

  return 'Change Talos version'
})
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex flex-col gap-1">
        <div
          class="flex items-center gap-2 text-xs text-naturals-n13"
          :class="{ 'opacity-50': machine.spec.tearing_down }"
        >
          <h2 class="list-item-link">
            <RouterLink :to="{ name: 'MachineLogs', params: { machine: machine.metadata.id } }">
              <WordHighlighter
                :query="searchQuery"
                split-by-space
                highlight-class="bg-naturals-n14"
              >
                {{ machineName }}
              </WordHighlighter>
            </RouterLink>
          </h2>

          <TIcon
            v-if="
              machine.spec.message_status?.power_state ===
              MachineStatusSpecPowerState.POWER_STATE_ON
            "
            icon="power"
            class="size-4 text-green-g1"
            aria-label="machine powered on"
          />

          <TIcon
            v-if="
              machine.spec.message_status?.power_state ===
              MachineStatusSpecPowerState.POWER_STATE_OFF
            "
            icon="power-off"
            class="size-4 text-red-r1"
            aria-label="machine powered off"
          />

          <CopyButton :text="machineName" />

          <div class="grow" />

          <Tooltip v-if="machine.spec.tearing_down" description="The machine is being destroyed">
            <TIcon icon="delete" class="h-4 w-4 text-red-r1" />
          </Tooltip>

          <div v-else class="flex gap-1">
            <Tooltip v-if="canAccessMaintenanceNodes" :description="maintenanceUpdateDescription">
              <IconButton
                icon="upgrade"
                :disabled="!canDoMaintenanceUpdate"
                @click="openMaintenanceUpdate"
              />
            </Tooltip>

            <!-- TODO: Extract logs button -->

            <TActionsBox class="h-6">
              <TActionsBoxItem icon="settings" @click="openConfigPatches">
                Config Patches
              </TActionsBoxItem>

              <TActionsBoxItem v-if="canReadMachineLogs" icon="log" @click="machineLogs">
                Logs
              </TActionsBoxItem>

              <TActionsBoxItem icon="copy" @click="copyMachineID">Copy Machine ID</TActionsBoxItem>

              <TActionsBoxItem
                v-if="clusterName && canReadClusters"
                icon="overview"
                @click="showCluster"
              >
                Show Cluster
              </TActionsBoxItem>

              <TActionsBoxItem v-if="canRemoveMachines" icon="delete" danger @click="removeMachine">
                Remove Machine
              </TActionsBoxItem>
            </TActionsBox>
          </div>
        </div>

        <ItemLabels
          :resource="machine"
          :add-label-func="machine.spec.tearing_down ? undefined : addMachineLabels"
          :remove-label-func="removeMachineLabels"
          @filter-label="(e) => $emit('filterLabels', e)"
        />
      </div>
    </template>

    <template #details>
      <div class="grid grid-cols-4 gap-2 pl-6">
        <MachineItemInfoCard
          title="Hardware"
          :sections="[
            { title: 'Processors', value: processors, emptyText: 'No processors detected' },
            { title: 'Memory', value: memorymodules, emptyText: 'No memory modules detected' },
            {
              title: 'Block devices',
              value: machine.spec.message_status?.hardware?.blockdevices?.map(
                (dev) => `${dev.linux_name} ${formatBytes(dev.size)} ${dev.type}`,
              ),
            },
            {
              title: 'UUID',
              value: () =>
                h(WordHighlighter, {
                  query: searchQuery ?? '',
                  splitBySpace: true,
                  textToHighlight: machine?.metadata?.id,
                  highlightClass: 'bg-naturals-n14',
                }),
            },
          ]"
        />

        <MachineItemInfoCard
          title="Network"
          :sections="[
            {
              title: 'Network interfaces',
              value: machine.spec.message_status?.network?.network_links?.map(
                (link) =>
                  `${link.linux_name} ${link.hardware_address} ${link.link_up ? 'UP' : 'DOWN'}`,
              ),
            },
            { title: 'Addresses', value: machine.spec.message_status?.network?.addresses },
            {
              title: 'Bytes sent',
              value: machine.spec.siderolink_counter?.bytes_sent
                ? formatBytes(machine.spec.siderolink_counter.bytes_sent)
                : '0B',
            },
            {
              title: 'Bytes received',
              value: machine.spec.siderolink_counter?.bytes_received
                ? formatBytes(machine.spec.siderolink_counter.bytes_received)
                : '0B',
            },
          ]"
        />

        <MachineItemInfoCard
          title="System"
          :sections="[
            { title: 'Extensions', value: machine.spec.message_status?.schematic?.extensions },
            { title: 'Patches', value: 'TODO: Get this value' },
            { title: 'PCI devices', value: 'TODO: Get this value' },
          ]"
        />

        <MachineItemInfoCard
          title="State"
          :sections="[
            { title: 'Created at', value: machineCreatedAt },
            { title: 'Last active', value: machineLastAlive },
            { title: 'Talos version', value: machine.spec.message_status?.talos_version },
            { title: 'Secure boot', value: secureBoot },
            {
              title: 'Cluster',
              value:
                clusterName &&
                (() =>
                  h(
                    RouterLink,
                    {
                      to: { name: 'ClusterOverview', params: { cluster: clusterName } },
                      class: 'list-item-link resource-label text-naturals-n12',
                    },
                    clusterName,
                  )),
            },
          ]"
        />
      </div>
    </template>
  </TListItem>
</template>
