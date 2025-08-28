<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { DateTime } from 'luxon'
import pluralize from 'pluralize'
import { computed, h } from 'vue'
import { RouterLink } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'
import { copyText } from 'vue3-clipboard'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusSpecPowerState } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelInstalled } from '@/api/resources'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
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
import type { Label } from '@/methods/labels'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import MachineItemInfoCard from '@/views/omni/Machines/MachineItemInfoCard.vue'

const { machine, searchQuery = '' } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
  searchQuery?: string
}>()

defineEmits<{
  filterLabels: [Label]
}>()

const selected = defineModel<boolean>('selected')

const machineName = computed(() => {
  return machine.spec.message_status?.network?.hostname ?? machine.metadata.id
})

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
  <TListItem v-model:selected="selected" is-selectable>
    <template #default>
      <div class="flex flex-col gap-1">
        <div
          class="flex items-center gap-2 text-xs text-naturals-n13"
          :class="{ 'opacity-50': machine.spec.tearing_down }"
        >
          <h2 class="list-item-link truncate">
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

          <CopyButton :text="machineName" />

          <TIcon
            v-if="
              machine.spec.message_status?.power_state ===
              MachineStatusSpecPowerState.POWER_STATE_ON
            "
            icon="power"
            class="size-4 shrink-0 text-green-g1"
            aria-label="machine powered on"
          />

          <TIcon
            v-if="
              machine.spec.message_status?.power_state ===
              MachineStatusSpecPowerState.POWER_STATE_OFF
            "
            icon="power-off"
            class="size-4 shrink-0 text-red-r1"
            aria-label="machine powered off"
          />

          <div class="grow" />

          <Tooltip v-if="machine.spec.tearing_down" description="The machine is being destroyed">
            <TIcon icon="delete" class="h-4 w-4 text-red-r1" />
          </Tooltip>

          <div v-else class="flex items-center gap-1">
            <RouterLink
              v-if="clusterName && canReadClusters"
              :to="{ name: 'ClusterOverview', params: { cluster: clusterName } }"
              class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:bg-naturals-n4 hover:text-naturals-n14"
            >
              <span class="max-md:hidden">{{ clusterName }}</span>
              <TIcon icon="clusters" aria-hidden="true" />
            </RouterLink>

            <RouterLink
              v-if="canReadMachineLogs"
              :to="{ name: 'MachineLogs', params: { machine: machine.metadata.id } }"
              class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:bg-naturals-n4 hover:text-naturals-n14"
            >
              <span class="max-md:hidden">Logs</span>
              <TIcon icon="log" aria-hidden="true" />
            </RouterLink>

            <Tooltip v-if="canAccessMaintenanceNodes" :description="maintenanceUpdateDescription">
              <button
                :disabled="!canDoMaintenanceUpdate"
                class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:not-disabled:bg-naturals-n4 hover:not-disabled:text-naturals-n14 disabled:opacity-40"
                @click="
                  $router.push({
                    query: {
                      modal: 'maintenanceUpdate',
                      machine: machine.metadata.id,
                      cluster: clusterName,
                    },
                  })
                "
              >
                <span class="max-md:hidden">Update Talos</span>
                <TIcon icon="upgrade" aria-hidden="true" />
              </button>
            </Tooltip>

            <TActionsBox class="h-6">
              <TActionsBoxItem
                icon="settings"
                @click="
                  $router.push({
                    name: 'MachineConfigPatches',
                    params: { machine: machine.metadata.id },
                  })
                "
              >
                Config Patches
              </TActionsBoxItem>

              <TActionsBoxItem
                icon="copy"
                @click="copyText(machine.metadata.id, undefined, () => {})"
              >
                Copy Machine ID
              </TActionsBoxItem>

              <TActionsBoxItem
                v-if="canRemoveMachines"
                icon="delete"
                danger
                @click="
                  $router.push({
                    query: {
                      modal: 'machineRemove',
                      machine: machine.metadata.id,
                      cluster: clusterName,
                    },
                  })
                "
              >
                Remove Machine
              </TActionsBoxItem>
            </TActionsBox>
          </div>
        </div>

        <ItemLabels
          :resource="machine"
          :add-label-func="machine.spec.tearing_down ? undefined : addMachineLabels"
          :remove-label-func="removeMachineLabels"
          @filter-label="(label) => $emit('filterLabels', label)"
        />
      </div>
    </template>

    <template #details>
      <div class="grid grid-cols-4 gap-2 pl-6 max-lg:grid-cols-2 max-md:grid-cols-1">
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
