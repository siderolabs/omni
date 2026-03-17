<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import { itemID } from '@/api/watch'
import TIcon, { type IconType } from '@/components/common/Icon/TIcon.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import type {
  TalosDiscoveredVolumeSpec,
  TalosVolumeStatusSpec,
} from '@/pages/(authenticated)/clusters/[cluster]/machine/[machine]/disks.vue'

defineProps<{
  partitions: {
    volume: Resource<TalosDiscoveredVolumeSpec>
    volumeStatus?: Resource<TalosVolumeStatusSpec>
  }[]
}>()

enum Encryption {
  Unknown = 'unknown',
  Enabled = 'enabled',
  Disabled = 'disabled',
}

const getFilesystemIcon = (fsType?: string): IconType => {
  if (!fsType) return 'question-mark-circle'
  const lower = fsType.toLowerCase()
  if (lower.includes('xfs') || lower.includes('ext')) return 'server'
  if (lower.includes('vfat') || lower.includes('fat')) return 'cpu-chip'
  return 'document'
}

const isEncrypted = (item?: Resource<TalosVolumeStatusSpec>) => {
  if (!item) {
    return Encryption.Unknown
  }

  return item.spec.encryptionProvider ? Encryption.Enabled : Encryption.Disabled
}

const getEncryptionClass = (item?: Resource<TalosVolumeStatusSpec>) => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'text-red-r1'
    case Encryption.Enabled:
      return 'text-green-g1'
    case Encryption.Unknown:
      return 'text-naturals-n11'
  }
}

const getEncryptionIcon = (item?: Resource<TalosVolumeStatusSpec>): IconType => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'unlocked'
    case Encryption.Enabled:
      return 'locked'
    case Encryption.Unknown:
      return 'question-mark-circle'
  }
}
</script>

<template>
  <div class="overflow-x-auto">
    <TableRoot class="w-full">
      <template #head>
        <TableRow>
          <TableCell th>Item</TableCell>
          <TableCell th>Filesystem</TableCell>
          <TableCell th>Encryption</TableCell>
          <TableCell th>UUID</TableCell>
          <TableCell th>Size</TableCell>
        </TableRow>
      </template>

      <template #body>
        <TableRow
          v-for="{ volume, volumeStatus } in partitions"
          :key="itemID(volume)"
          :aria-labelledby="`volume-${volume.metadata.id}-title`"
          class="whitespace-nowrap"
        >
          <TableCell>
            <div class="flex items-center gap-2">
              <TIcon
                :icon="getFilesystemIcon(volume.spec.name || volumeStatus?.spec.filesystem)"
                class="size-4 shrink-0 text-naturals-n14"
              />

              <div>
                <div
                  :id="`volume-${volume.metadata.id}-title`"
                  class="max-w-40 truncate font-medium text-naturals-n14"
                  :title="volume.spec.partition_label || volume.spec.label || volume.metadata.id"
                >
                  {{ volume.spec.partition_label || volume.spec.label || volume.metadata.id }}
                </div>
                <div class="text-xs text-naturals-n10">
                  {{ volume.spec.dev_path }}
                </div>
              </div>
            </div>
          </TableCell>

          <TableCell class="font-medium">
            {{ volume.spec.name || volumeStatus?.spec.filesystem || 'N/A' }}
          </TableCell>

          <TableCell>
            <span
              class="inline-flex items-center gap-1 rounded bg-naturals-n4 px-1.5 py-0.75"
              :class="getEncryptionClass(volumeStatus)"
            >
              <TIcon :icon="getEncryptionIcon(volumeStatus)" class="size-3" aria-hidden />
              <span class="text-xs/none">{{ isEncrypted(volumeStatus) }}</span>
            </span>
          </TableCell>

          <TableCell class="w-full max-w-20 truncate font-medium" :title="volume.spec.uuid">
            {{ volume.spec.uuid }}
          </TableCell>

          <TableCell class="font-medium">{{ volume.spec.pretty_size }}</TableCell>
        </TableRow>
      </template>
    </TableRoot>
  </div>
</template>
