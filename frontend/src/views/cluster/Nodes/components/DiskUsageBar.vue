<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import { itemID } from '@/api/watch'
import { formatBytes } from '@/methods'
import type { TalosDiscoveredVolumeSpec, TalosDiskSpec } from '@/views/cluster/Nodes/NodeDisks.vue'

const { disk, volumes } = defineProps<{
  disk: Resource<TalosDiskSpec>
  volumes: Resource<TalosDiscoveredVolumeSpec>[]
}>()

const usedSpace = computed(() => volumes.reduce((sum, p) => sum + (p.spec.size || 0), 0))

const unallocatedSpace = computed(() => (disk.spec.size || 0) - usedSpace.value)

const partitionPercent = (partitionSize?: number, diskSize?: number) => {
  if (!partitionSize || !diskSize) return 0
  return (partitionSize / diskSize) * 100
}

const unallocatedPercent = computed(() => {
  const diskSize = disk.spec.size || 0

  return Math.max(0, partitionPercent(diskSize - usedSpace.value, diskSize))
})

const getVolumeClass = (volume: Resource<TalosDiscoveredVolumeSpec>) => {
  const fsType = volume.spec.name?.toLowerCase()
  const label = volume.spec.partition_label?.toLowerCase()

  if (label?.includes('efi')) return 'bg-yellow-500'
  if (label?.includes('boot')) return 'bg-orange-500'
  if (label?.includes('state')) return 'bg-purple-500'
  if (label?.includes('ephemeral')) return 'bg-blue-500'
  if (fsType?.includes('xfs') || fsType?.includes('ext')) return 'bg-green-600'
  if (fsType?.includes('vfat') || fsType?.includes('fat')) return 'bg-yellow-600'
  if (fsType?.includes('swap')) return 'bg-red-500'
  return 'bg-naturals-n9'
}
</script>

<template>
  <div>
    <div class="flex h-8 w-full overflow-hidden rounded bg-naturals-n5">
      <div
        v-for="volume in volumes"
        :key="itemID(volume)"
        :title="`${volume.spec.partition_label || volume.spec.label || volume.spec.dev_path} — ${volume.spec.pretty_size}`"
        :class="getVolumeClass(volume)"
        :style="{
          width: `${partitionPercent(volume.spec.size, disk.spec.size)}%`,
        }"
        class="flex min-w-0 items-center justify-center overflow-hidden border-r border-naturals-n3/50 text-xs text-white last:border-r-0"
      >
        <span
          v-if="partitionPercent(volume.spec.size, disk.spec.size) > 10"
          class="truncate px-1.5 text-xs font-medium drop-shadow-xs drop-shadow-black"
        >
          {{ volume.spec.partition_label || volume.spec.label || '' }}
          <span class="opacity-80">
            {{ volume.spec.pretty_size }}
          </span>
        </span>
      </div>
      <div
        v-if="unallocatedPercent > 0"
        :style="{ width: `${unallocatedPercent}%` }"
        :title="`Unallocated — ${formatBytes(unallocatedSpace)}`"
        class="flex min-w-0 items-center justify-center overflow-hidden text-xs text-naturals-n11"
      >
        <span v-if="unallocatedPercent > 10" class="truncate px-1.5">Unallocated</span>
      </div>
    </div>

    <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-naturals-n11">
      <div
        v-for="volume in volumes"
        :key="'legend-' + itemID(volume)"
        class="flex items-center gap-1.5"
      >
        <span class="inline-block size-2.5 rounded-sm" :class="getVolumeClass(volume)" />
        <span>
          {{ volume.spec.partition_label || volume.spec.label || volume.spec.dev_path }}
        </span>
        <span class="text-naturals-n9">
          {{ volume.spec.pretty_size }}
        </span>
      </div>
      <div v-if="unallocatedPercent > 0" class="flex items-center gap-1.5">
        <span class="inline-block size-2.5 rounded-sm bg-naturals-n5" />
        <span>Unallocated</span>
        <span class="text-naturals-n9">
          {{ formatBytes(unallocatedSpace) }}
        </span>
      </div>
    </div>
  </div>
</template>
