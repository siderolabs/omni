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

export interface TalosVolumeStatusMountSpec {
  fileMode?: number
  gid?: number
  parentId?: string
  projectQuotaSupport?: boolean
  recursiveRelabel?: boolean
  selinuxLabel?: string
  targetPath?: string
  uid?: number
}

export interface TalosVolumeStatusSpec {
  mountSpec?: TalosVolumeStatusMountSpec
  phase?: string
  type?: string
  parentID?: string
  symlink?: {
    force?: boolean
    symlinkTargetPath?: string
  }
  configuredEncryptionKeys?: string[]
  encryptionProvider?: string
  encryptionSlot?: number
  filesystem?: string
  location?: string
  mountLocation?: string
  parentLocation?: string
  partitionIndex?: number
  partitionUUID?: string
  prettySize?: string
  size?: number
  uuid?: string
}
</script>

<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import {
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosRuntimeNamespace,
  TalosVolumeStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'
import DiskPartitionTable from '@/views/cluster/Nodes/components/DiskPartitionTable.vue'
import DiskUsageBar from '@/views/cluster/Nodes/components/DiskUsageBar.vue'

definePage({ name: 'NodeDisks' })

const context = getContext()

const {
  data: disks,
  loading: disksLoading,
  err: disksErr,
  errCode: disksErrCode,
} = useResourceWatch<TalosDiskSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiskType,
  },
  runtime: Runtime.Talos,
  context,
}))

const {
  data: volumes,
  loading: volumesLoading,
  err: volumesErr,
  errCode: volumesErrCode,
} = useResourceWatch<TalosDiscoveredVolumeSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiscoveredVolumeType,
  },
  runtime: Runtime.Talos,
  context,
}))

const {
  data: volumeStatuses,
  loading: volumeStatusesLoading,
  err: volumeStatusesErr,
  errCode: volumeStatusesErrCode,
} = useResourceWatch<TalosVolumeStatusSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosVolumeStatusType,
  },
  runtime: Runtime.Talos,
  context,
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
    .filter((d) => !d.metadata.id?.startsWith('loop'))
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
    })),
)
</script>

<template>
  <PageContainer class="space-y-4">
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
      :key="itemID(diskInfo.disk)"
      class="overflow-hidden rounded-lg"
      :aria-labelledby="`disk-${diskInfo.disk.metadata.id}-title`"
    >
      <div class="border-b border-naturals-n6 bg-naturals-n3 p-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <TIcon icon="server" class="size-6 text-naturals-n13" />
            <div class="space-y-1 text-sm/none">
              <p :id="`disk-${diskInfo.disk.metadata.id}-title`" class="text-naturals-n14">
                {{ diskInfo.disk.metadata.id }}
              </p>
              <p class="font-medium text-naturals-n11">
                {{ diskInfo.disk.spec.dev_path }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-4">
            <span class="text-sm text-naturals-n14">
              {{ diskInfo.disk.spec.pretty_size }}
            </span>
            <div class="flex items-center gap-2">
              <span
                v-if="diskInfo.disk.spec.transport"
                class="rounded bg-naturals-n5 px-2 py-1 text-xs text-naturals-n13"
              >
                {{ diskInfo.disk.spec.transport }}
              </span>
              <span
                v-if="diskInfo.disk.spec.readonly"
                class="rounded bg-yellow-y1/20 px-2 py-1 text-xs text-yellow-y1"
              >
                Read-only
              </span>
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-2 bg-naturals-n2 p-4">
        <DiskUsageBar :disk="diskInfo.disk" :volumes="diskInfo.partitions.map((p) => p.volume)" />
        <DiskPartitionTable v-if="diskInfo.partitions.length" :partitions="diskInfo.partitions" />
      </div>
    </section>
  </PageContainer>
</template>
