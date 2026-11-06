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
import {
  getDiskSegments,
  getVolumeDisplayName,
  isClaimedVolume,
} from '@/views/Machines/lib/diskVolumes'

const { disk, volumes } = defineProps<{
  disk: Resource<DiskSpec>
  volumes: Resource<DiscoveredVolumeSpec>[]
}>()

const segments = computed(() => getDiskSegments(disk.spec.size || 0, volumes))

const unallocatedSpace = computed(() =>
  segments.value.reduce((sum, s) => (s.type === 'unallocated' ? sum + s.size : sum), 0),
)

const segmentPercent = (size: number) => {
  const diskSize = disk.spec.size || 0

  if (!diskSize) return 100
  return (size / diskSize) * 100
}

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
          if (isClaimedVolume(v)) return false

          const label = v.spec.partition_label?.toLowerCase()

          return !label || !(label in knownLabelColors)
        })
        .map(
          (v, i) =>
            [v.metadata.id!, otherLabelColors[i] ?? 'bg-series-8 text-on-series-8'] as const,
        ),
    ),
)

// Space handed over to another device (an MD array, an LVM volume group) is shown
// again on that device, so it is drawn neutral here rather than as data.
const claimedVolumeClass =
  'bg-surface-raised text-content-secondary ring-1 ring-border-strong ring-inset'

const getVolumeClass = (volume: Resource<DiscoveredVolumeSpec>) => {
  if (isClaimedVolume(volume)) return claimedVolumeClass

  const label = volume.spec.partition_label?.toLowerCase() as keyof typeof knownLabelColors

  return (label && knownLabelColors[label]) || otherVolumeColors.value.get(volume.metadata.id!)
}
</script>

<template>
  <div>
    <div class="flex h-8 w-full overflow-hidden rounded bg-surface-inert text-xs">
      <template v-for="segment in segments">
        <div
          v-if="segment.type === 'volume'"
          :key="segment.volume.metadata.id"
          :title="`${getVolumeDisplayName(segment.volume)} — ${prettyBytes(segment.size)}`"
          :class="getVolumeClass(segment.volume)"
          :style="{ width: `${segmentPercent(segment.size)}%` }"
          class="flex min-w-0 items-center justify-center overflow-hidden"
        >
          <span v-if="segmentPercent(segment.size) > 10" class="truncate px-micro font-medium">
            {{ getVolumeDisplayName(segment.volume) }}
            <span>
              {{ prettyBytes(segment.size) }}
            </span>
          </span>
        </div>

        <div
          v-else
          :key="`unallocated-${segment.offset}`"
          :style="{ width: `${segmentPercent(segment.size)}%` }"
          :title="`Unallocated — ${prettyBytes(segment.size)}`"
          class="flex min-w-0 items-center justify-center overflow-hidden bg-[repeating-linear-gradient(-45deg,var(--stripe-color),var(--stripe-color)_var(--stripe-size),transparent_var(--stripe-size),transparent_calc(var(--stripe-size)*2))] text-content-default [--stripe-color:var(--color-surface-hover)] [--stripe-size:12px]"
        >
          <span v-if="segmentPercent(segment.size) > 10" class="truncate px-micro font-medium">
            Unallocated
          </span>
        </div>
      </template>
    </div>

    <div class="mt-tight flex flex-wrap gap-x-compact gap-y-micro text-xs text-content-secondary">
      <div
        v-for="volume in volumes"
        :key="'legend-' + volume.metadata.id"
        class="flex items-center gap-micro"
      >
        <span class="inline-block size-2.5 rounded-sm" :class="getVolumeClass(volume)" />
        <span class="font-medium text-content-default">
          {{ getVolumeDisplayName(volume) }}
        </span>
        <span class="font-medium text-content-muted">
          {{ prettyBytes(volume.spec.size ?? 0) }}
        </span>
      </div>
      <div v-if="unallocatedSpace > 0" class="flex items-center gap-micro">
        <span class="inline-block size-2.5 rounded-sm bg-surface-inert" />
        <span class="font-medium text-content-default">Unallocated</span>
        <span class="font-medium text-content-muted">
          {{ prettyBytes(unallocatedSpace) }}
        </span>
      </div>
    </div>
  </div>
</template>
