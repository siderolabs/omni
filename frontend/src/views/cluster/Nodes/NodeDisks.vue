<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface TalosDiskSpec {
  bus_path?: string
  cdrom?: boolean
  dev_path?: string
  io_size?: number
  modalias?: string
  pretty_size?: string
  readonly?: boolean
  rotational?: boolean
  sector_size?: number
  size?: number
  sub_system?: string
  symlinks?: string[]
  transport?: string
}

export interface TalosDiscoveredVolumeSpec {
  block_size?: number
  dev_path?: string
  device_path?: string
  filesystem_block_size?: number
  io_size?: number
  name?: string
  label?: string
  offset?: number
  parent?: string
  parent_dev_path?: string
  partition_index?: number
  partition_label?: string
  partition_type?: string
  partition_uuid?: string
  pretty_size?: string
  probed_size?: number
  sector_size?: number
  size?: number
  type?: string
  uuid?: string
}

export interface TalosMountStatusSpec {
  encrypted?: boolean
  filesystemType?: string
  source?: string
  target?: string
}
</script>
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import {
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosMountStatusType,
  TalosRuntimeNamespace,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'
import DiskUsageBar from '@/views/cluster/Nodes/components/DiskUsageBar.vue'
import DiskVolumeCard from '@/views/cluster/Nodes/components/DiskVolumeCard.vue'

const context = getContext()

const { data: disks } = useResourceWatch<TalosDiskSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiskType,
  },
  runtime: Runtime.Talos,
  context,
}))

const { data: volumes } = useResourceWatch<TalosDiscoveredVolumeSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiscoveredVolumeType,
  },
  runtime: Runtime.Talos,
  context,
}))

const { data: mounts } = useResourceWatch<TalosMountStatusSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosMountStatusType,
  },
  runtime: Runtime.Talos,
  context,
}))

const organizedDisks = computed(() =>
  disks.value
    .filter((d) => !d.metadata.id?.startsWith('loop'))
    .map((d) => ({
      disk: d,
      partitions: volumes.value
        .filter((v) => v.spec.parent === d.metadata.id)
        .map((v) => ({
          volume: v,
          mount: mounts.value.find((m) => m.spec.source === v.spec.dev_path),
        }))
        .sort(
          (a, b) => (a.volume.spec.partition_index || 0) - (b.volume.spec.partition_index || 0),
        ),
    })),
)
</script>

<template>
  <div class="space-y-4">
    <TAlert v-if="!organizedDisks.length" type="info" title="No Records">No disks found.</TAlert>

    <section
      v-for="diskInfo in organizedDisks"
      :key="itemID(diskInfo.disk)"
      class="overflow-hidden rounded-lg border border-naturals-n7 bg-naturals-n3"
      :aria-labelledby="`disk-${diskInfo.disk.metadata.id}-title`"
    >
      <div class="border-b border-naturals-n7 bg-naturals-n4 px-4 py-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <TIcon icon="server" class="size-5 text-naturals-n13" />
            <div>
              <div
                :id="`disk-${diskInfo.disk.metadata.id}-title`"
                class="font-medium text-naturals-n14"
              >
                {{ diskInfo.disk.metadata.id }}
              </div>
              <div class="text-sm text-naturals-n13">
                {{ diskInfo.disk.spec.dev_path }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-4 text-sm">
            <div class="text-naturals-n13">
              <span class="text-naturals-n11">Size:</span>
              <span class="ml-1 font-medium text-naturals-n14">
                {{ diskInfo.disk.spec.pretty_size }}
              </span>
            </div>
            <div
              v-if="diskInfo.disk.spec.transport"
              class="rounded bg-naturals-n5 px-2 py-1 text-naturals-n13"
            >
              {{ diskInfo.disk.spec.transport }}
            </div>
            <div
              v-if="diskInfo.disk.spec.readonly"
              class="rounded bg-yellow-y1/20 px-2 py-1 text-yellow-y1"
            >
              Read-only
            </div>
          </div>
        </div>
      </div>

      <TAlert v-if="!diskInfo.partitions.length" type="info" title="No Records" class="m-4">
        No partitions found for
        <code>{{ diskInfo.disk.metadata.id }}</code>
      </TAlert>

      <div v-else class="space-y-4 p-4">
        <DiskUsageBar :disk="diskInfo.disk" :volumes="diskInfo.partitions.map((p) => p.volume)" />

        <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <DiskVolumeCard
            v-for="partition in diskInfo.partitions"
            :key="itemID(partition.volume)"
            :volume="partition.volume"
            :mount="partition.mount"
          />
        </div>
      </div>
    </section>
  </div>
</template>
M
