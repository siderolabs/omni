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

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { formatBytes } from '@/methods'
import MachineItemInfoCard from '@/views/omni/Machines/MachineItemInfoCard.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const { machine = undefined, searchQuery = '' } = defineProps<{
  machine?: Resource<MachineStatusLinkSpec>
  searchQuery?: string
}>()

defineEmits<{
  close: []
}>()

const machineName = computed(
  () => machine?.spec.message_status?.network?.hostname ?? machine?.metadata.id,
)

const processors = computed(() =>
  machine?.spec.message_status?.hardware?.processors?.map(
    ({ frequency, core_count, description }) => {
      const parts: string[] = []

      if (frequency) parts.push(`${frequency / 1000} GHz`)
      if (core_count) parts.push(pluralize('core', core_count, true))
      if (description?.trim()) parts.push(description.trim())

      return parts.join(', ')
    },
  ),
)

const memorymodules = computed(() =>
  machine?.spec.message_status?.hardware?.memory_modules
    ?.filter((mem) => !!mem.size_mb)
    .map(({ description, size_mb = 0 }) => `${formatBytes(size_mb * 1024 * 1024)} ${description}`),
)

const clusterName = computed(() => machine?.spec.message_status?.cluster)

const formatTime = (time?: string) => {
  if (!time) return 'Never'

  const dt = /^\d+$/.test(time) ? DateTime.fromSeconds(parseInt(time)) : DateTime.fromISO(time)

  return dt.setLocale('en').toLocaleString(DateTime.DATETIME_FULL_WITH_SECONDS)
}

const machineLastAlive = computed(() => formatTime(machine?.spec.siderolink_counter?.last_alive))
const machineCreatedAt = computed(() => formatTime(machine?.spec.machine_created_at))
const secureBoot = computed(() => {
  const securityState = machine?.spec.message_status?.security_state
  if (!securityState) return 'Unknown'

  return securityState.secure_boot ? 'Enabled' : 'Disabled'
})
</script>

<template>
  <div class="flex flex-col gap-2 border-l-naturals-n4 bg-naturals-n0 p-4 md:border-l">
    <div class="flex justify-between gap-2">
      <h2 class="truncate font-medium text-naturals-n14">{{ machineName }}</h2>

      <CloseButton class="shrink-0" @click="$emit('close')" />
    </div>

    <!-- pr-1 to give some padding between bg & scrollbar -->
    <div class="flex flex-col gap-2 overflow-auto pr-1">
      <MachineItemInfoCard
        title="Hardware"
        :sections="[
          { title: 'Processors', value: processors, emptyText: 'No processors detected' },
          { title: 'Memory', value: memorymodules, emptyText: 'No memory modules detected' },
          {
            title: 'Block devices',
            value: machine?.spec.message_status?.hardware?.blockdevices?.map(
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
            value: machine?.spec.message_status?.network?.network_links?.map(
              (link) =>
                `${link.linux_name} ${link.hardware_address} ${link.link_up ? 'UP' : 'DOWN'}`,
            ),
          },
          { title: 'Addresses', value: machine?.spec.message_status?.network?.addresses },
          {
            title: 'Bytes sent',
            value: machine?.spec.siderolink_counter?.bytes_sent
              ? formatBytes(machine?.spec.siderolink_counter.bytes_sent)
              : '0B',
          },
          {
            title: 'Bytes received',
            value: machine?.spec.siderolink_counter?.bytes_received
              ? formatBytes(machine?.spec.siderolink_counter.bytes_received)
              : '0B',
          },
        ]"
      />

      <MachineItemInfoCard
        title="System"
        :sections="[
          { title: 'Extensions', value: machine?.spec.message_status?.schematic?.extensions },
        ]"
      />

      <MachineItemInfoCard
        title="State"
        :sections="[
          { title: 'Created at', value: machineCreatedAt },
          { title: 'Last active', value: machineLastAlive },
          { title: 'Talos version', value: machine?.spec.message_status?.talos_version },
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
  </div>
</template>
