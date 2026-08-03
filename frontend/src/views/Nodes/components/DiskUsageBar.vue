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
  efi: 'bg-yellow-500',
  boot: 'bg-orange-500',
  state: 'bg-purple-500',
  ephemeral: 'bg-blue-500',
}

const unknownLabelColors = computed(
  () =>
    new Map(
      volumes
        .filter((v) => {
          const label = v.spec.partition_label?.toLowerCase()

          return !label || !(label in knownLabelColors)
        })
        .map((v, i) => {
          const hue = ((i + 1) * 137.508) % 360

          return [v.metadata.id!, `hsl(${hue.toFixed(1)}deg 55% 30%)`] as const
        }),
    ),
)

const getVolumeClass = (volume: Resource<DiscoveredVolumeSpec>) => {
  const label = volume.spec.partition_label?.toLowerCase() as keyof typeof knownLabelColors

  return label ? knownLabelColors[label] : undefined
}

const getVolumeStyle = (volume: Resource<DiscoveredVolumeSpec>) => {
  const color = unknownLabelColors.value.get(volume.metadata.id!)

  return color ? { backgroundColor: color } : undefined
}
</script>

<template>
  <div>
    <div class="flex h-8 w-full overflow-hidden rounded bg-naturals-n5 text-xs">
      <div
        v-for="volume in volumes"
        :key="volume.metadata.id"
        :title="`${volume.spec.partition_label || volume.spec.label || volume.spec.dev_path} — ${prettyBytes(volume.spec.size ?? 0)}`"
        :class="getVolumeClass(volume)"
        :style="{
          width: `${partitionPercent(volume.spec.size, disk.spec.size)}%`,
          ...getVolumeStyle(volume),
        }"
        class="flex min-w-0 items-center justify-center overflow-hidden text-naturals-n14 last:border-r-0"
      >
        <span
          v-if="partitionPercent(volume.spec.size, disk.spec.size) > 10"
          class="truncate px-1.5 font-medium drop-shadow-xs drop-shadow-black"
        >
          {{ volume.spec.partition_label || volume.spec.label || '' }}
          <span class="opacity-80">
            {{ prettyBytes(volume.spec.size ?? 0) }}
          </span>
        </span>
      </div>

      <div
        v-if="unallocatedPercent > 0"
        :style="{ width: `${unallocatedPercent}%` }"
        title="Unallocated"
        class="flex min-w-0 items-center justify-center overflow-hidden bg-[repeating-linear-gradient(-45deg,var(--stripe-color),var(--stripe-color)_var(--stripe-size),transparent_var(--stripe-size),transparent_calc(var(--stripe-size)*2))] text-naturals-n12 [--stripe-color:var(--color-naturals-n7)] [--stripe-size:12px]"
      >
        <span
          v-if="unallocatedPercent > 10"
          class="truncate px-1.5 font-medium drop-shadow-xs drop-shadow-black"
        >
          Unallocated
        </span>
      </div>
    </div>

    <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-naturals-n11">
      <div
        v-for="volume in volumes"
        :key="'legend-' + volume.metadata.id"
        class="flex items-center gap-1.5"
      >
        <span
          class="inline-block size-2.5 rounded-sm"
          :class="getVolumeClass(volume)"
          :style="getVolumeStyle(volume)"
        />
        <span class="font-medium text-naturals-n12">
          {{ volume.spec.partition_label || volume.spec.label || volume.spec.dev_path }}
        </span>
        <span class="font-medium text-naturals-n10">
          {{ prettyBytes(volume.spec.size ?? 0) }}
        </span>
      </div>
      <div v-if="unallocatedPercent > 0" class="flex items-center gap-1.5">
        <span class="inline-block size-2.5 rounded-sm bg-naturals-n5" />
        <span class="font-medium text-naturals-n12">Unallocated</span>
        <span class="font-medium text-naturals-n10">
          {{ prettyBytes(unallocatedSpace) }}
        </span>
      </div>
    </div>
  </div>
</template>
