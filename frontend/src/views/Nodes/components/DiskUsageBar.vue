<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import prettyBytes from 'pretty-bytes'
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { DiscoveredVolumeSpec, DiskSpec } from '@/api/talos/block.pb'

const { disk, volumes } = defineProps<{
  disk: Resource<DiskSpec>
  volumes: Resource<DiscoveredVolumeSpec>[]
}>()

const usedSpace = computed(() => volumes.reduce((sum, p) => sum + (p.spec.size || 0), 0))

const unallocatedSpace = computed(() => (disk.spec.size || 0) - usedSpace.value)

const partitionPercent = (partitionSize?: number, diskSize?: number) => {
  if (!diskSize) return 100
  if (!partitionSize) return 0
  return (partitionSize / diskSize) * 100
}

const unallocatedPercent = computed(() => {
  const diskSize = disk.spec.size || 0

  return Math.max(0, partitionPercent(diskSize - usedSpace.value, diskSize))
})

const knownLabelColors = {
  ephemeral: 'bg-series-1 text-on-series-1',
  state: 'bg-series-2 text-on-series-2',
  efi: 'bg-series-3 text-on-series-3',
  boot: 'bg-series-4 text-on-series-4',
  meta: 'bg-series-5 text-on-series-5',
}

const otherLabelColors = ['bg-series-6 text-on-series-6', 'bg-series-7 text-on-series-7']

const otherVolumeColors = computed(
  () =>
    new Map(
      volumes
        .filter((v) => {
          const label = v.spec.partition_label?.toLowerCase()

          return !label || !(label in knownLabelColors)
        })
        .map(
          (v, i) =>
            [v.metadata.id!, otherLabelColors[i] ?? 'bg-series-8 text-on-series-8'] as const,
        ),
    ),
)

const getVolumeClass = (volume: Resource<DiscoveredVolumeSpec>) => {
  const label = volume.spec.partition_label?.toLowerCase() as keyof typeof knownLabelColors

  return (label && knownLabelColors[label]) || otherVolumeColors.value.get(volume.metadata.id!)
}
</script>

<template>
  <div>
    <div class="flex h-8 w-full overflow-hidden rounded bg-surface-inert text-xs">
      <div
        v-for="volume in volumes"
        :key="volume.metadata.id"
        :title="`${volume.spec.partition_label || volume.spec.label || volume.spec.dev_path} — ${prettyBytes(volume.spec.size ?? 0)}`"
        :class="getVolumeClass(volume)"
        :style="{
          width: `${partitionPercent(volume.spec.size, disk.spec.size)}%`,
        }"
        class="flex min-w-0 items-center justify-center overflow-hidden last:border-r-0"
      >
        <span
          v-if="partitionPercent(volume.spec.size, disk.spec.size) > 10"
          class="truncate px-1.5 font-medium"
        >
          {{ volume.spec.partition_label || volume.spec.label || '' }}
          <span>
            {{ prettyBytes(volume.spec.size ?? 0) }}
          </span>
        </span>
      </div>

      <div
        v-if="unallocatedPercent > 0"
        :style="{ width: `${unallocatedPercent}%` }"
        title="Unallocated"
        class="flex min-w-0 items-center justify-center overflow-hidden bg-[repeating-linear-gradient(-45deg,var(--stripe-color),var(--stripe-color)_var(--stripe-size),transparent_var(--stripe-size),transparent_calc(var(--stripe-size)*2))] text-content-default [--stripe-color:var(--color-surface-hover)] [--stripe-size:12px]"
      >
        <span v-if="unallocatedPercent > 10" class="truncate px-1.5 font-medium">Unallocated</span>
      </div>
    </div>

    <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-content-secondary">
      <div
        v-for="volume in volumes"
        :key="'legend-' + volume.metadata.id"
        class="flex items-center gap-1.5"
      >
        <span class="inline-block size-2.5 rounded-sm" :class="getVolumeClass(volume)" />
        <span class="font-medium text-content-default">
          {{ volume.spec.partition_label || volume.spec.label || volume.spec.dev_path }}
        </span>
        <span class="font-medium text-content-muted">
          {{ prettyBytes(volume.spec.size ?? 0) }}
        </span>
      </div>
      <div v-if="unallocatedPercent > 0" class="flex items-center gap-1.5">
        <span class="inline-block size-2.5 rounded-sm bg-surface-inert" />
        <span class="font-medium text-content-default">Unallocated</span>
        <span class="font-medium text-content-muted">
          {{ prettyBytes(unallocatedSpace) }}
        </span>
      </div>
    </div>
  </div>
</template>
