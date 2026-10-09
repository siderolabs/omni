<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import prettyBytes from 'pretty-bytes'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { RuntimeContext } from '@/api/options'
import {
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosRuntimeNamespace,
  TalosVolumeStatusType,
} from '@/api/resources'
import type { DiscoveredVolumeSpec, DiskSpec, VolumeStatusSpec } from '@/api/talos/block.pb'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import DiskPartitionTable from '@/views/Nodes/components/DiskPartitionTable.vue'
import DiskUsageBar from '@/views/Nodes/components/DiskUsageBar.vue'

const { machineId } = defineProps<{
  machineId: string
  clusterId?: string
}>()

const context = computed<RuntimeContext>(() => ({
  node: machineId,
}))

const {
  data: disks,
  loading: disksLoading,
  err: disksErr,
  errCode: disksErrCode,
} = useResourceWatch<DiskSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiskType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const {
  data: volumes,
  loading: volumesLoading,
  err: volumesErr,
  errCode: volumesErrCode,
} = useResourceWatch<DiscoveredVolumeSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiscoveredVolumeType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const {
  data: volumeStatuses,
  loading: volumeStatusesLoading,
  err: volumeStatusesErr,
  errCode: volumeStatusesErrCode,
} = useResourceWatch<VolumeStatusSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosVolumeStatusType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const loading = computed(
  () => disksLoading.value || volumesLoading.value || volumeStatusesLoading.value,
)

const err = computed(() => disksErr.value || volumesErr.value || volumeStatusesErr.value)
const errCode = computed(
  () => disksErrCode.value || volumesErrCode.value || volumeStatusesErrCode.value,
)

const organizedDisks = computed(() =>
  disks.value
    .filter(
      (d) =>
        !d.metadata.id?.startsWith('loop') &&
        // Device-mapper devices (dm-N) are synthetic kernel block devices created for
        // things like LUKS encryption. They are represented through their parent
        // partition's VolumeStatus and should not appear as top-level disk entries.
        !d.metadata.id?.startsWith('dm-'),
    )
    .map((disk) => ({
      disk,
      partitions: volumes.value
        .filter((v) => v.spec.parent === disk.metadata.id)
        .map((v) => ({
          volume: v,
          volumeStatus: volumeStatuses.value.find((m) => m.spec.location === v.spec.dev_path),
        }))
        .sort(
          (a, b) => (a.volume.spec.partition_index || 0) - (b.volume.spec.partition_index || 0),
        ),
    }))
    // Hide empty CD-ROMs. Talos always creates a DiscoveredVolume for a CDROM
    // drive but leaves `name` (filesystem type) as "" when no media is present.
    // The kernel does not partition CD-ROMs, so the name is on the whole-disk volume.
    .filter(
      (d) =>
        !d.disk.spec.cdrom ||
        volumes.value.some((v) => v.metadata.id === d.disk.metadata.id && v.spec.name),
    ),
)
</script>

<template>
  <PageContainer class="space-y-compact">
    <TAlert v-if="errCode === Code.UNAVAILABLE" type="warn" title="Machine not ready">
      Talos API is not ready yet
    </TAlert>
    <TSpinner v-else-if="loading" class="mx-auto size-6" />
    <TAlert v-else-if="err" type="error" title="Error">{{ err }}</TAlert>
    <TAlert v-else-if="!organizedDisks.length" type="info" title="No Records">
      No disks found.
    </TAlert>

    <section
      v-for="diskInfo in organizedDisks"
      v-else
      :key="diskInfo.disk.metadata.id"
      class="overflow-hidden rounded-lg border border-border-default bg-surface-card"
      :aria-labelledby="`disk-${diskInfo.disk.metadata.id}-title`"
    >
      <div class="border-b border-border-strong bg-surface-raised p-compact">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-snug">
            <TIcon icon="server" class="size-6 text-content-default" />
            <div class="space-y-micro text-sm/none">
              <p :id="`disk-${diskInfo.disk.metadata.id}-title`" class="text-content-emphasis">
                {{ diskInfo.disk.metadata.id }}
              </p>
              <p class="font-medium text-content-secondary">
                {{ diskInfo.disk.spec.dev_path }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-compact">
            <span class="text-sm text-content-emphasis">
              {{ prettyBytes(diskInfo.disk.spec.size ?? 0) }}
            </span>
            <div class="flex items-center gap-tight">
              <span
                v-if="diskInfo.disk.spec.cdrom"
                class="rounded bg-surface-inert px-tight py-micro text-xs text-content-default"
              >
                CD-ROM
              </span>
              <span
                v-if="diskInfo.disk.spec.transport"
                class="rounded bg-surface-inert px-tight py-micro text-xs text-content-default"
              >
                {{ diskInfo.disk.spec.transport }}
              </span>
              <span
                v-if="diskInfo.disk.spec.readonly"
                class="rounded bg-status-warning-subtle px-tight py-micro text-xs text-status-warning-text ring-1 ring-status-warning-subtle-border ring-inset"
              >
                Read-only
              </span>
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-tight bg-surface-card p-compact">
        <DiskUsageBar :disk="diskInfo.disk" :volumes="diskInfo.partitions.map((p) => p.volume)" />
        <DiskPartitionTable v-if="diskInfo.partitions.length" :partitions="diskInfo.partitions" />
      </div>
    </section>
  </PageContainer>
</template>
