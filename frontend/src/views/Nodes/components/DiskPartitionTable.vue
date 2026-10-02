<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import prettyBytes from 'pretty-bytes'

import type { Resource } from '@/api/grpc'
import type { DiscoveredVolumeSpec, VolumeStatusSpec } from '@/api/talos/block.pb'
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'

defineProps<{
  partitions: {
    volume: Resource<DiscoveredVolumeSpec>
    volumeStatus?: Resource<VolumeStatusSpec>
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
  if (lower.includes('luks')) return 'locked'
  return 'document'
}

const getEffectiveFilesystem = (
  volume: Resource<DiscoveredVolumeSpec>,
  volumeStatus?: Resource<VolumeStatusSpec>,
): string => {
  // When encryption is active, the DV name is luks, but we want to show the actual filesystem
  if (volumeStatus?.spec.encryptionProvider && volumeStatus.spec.filesystem) {
    return volumeStatus.spec.filesystem
  }
  return volume.spec.name || volumeStatus?.spec.filesystem || 'N/A'
}

const isEncrypted = (item?: Resource<VolumeStatusSpec>) => {
  if (!item) {
    return Encryption.Unknown
  }

  return item.spec.encryptionProvider ? Encryption.Enabled : Encryption.Disabled
}

const getEncryptionClass = (item?: Resource<VolumeStatusSpec>) => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'bg-surface-hover text-content-default'
    case Encryption.Enabled:
      return 'bg-status-success-subtle text-status-success-text ring-1 ring-status-success-subtle-border ring-inset'
    case Encryption.Unknown:
      return 'bg-surface-hover text-content-secondary'
  }
}

const getEncryptionIcon = (item?: Resource<VolumeStatusSpec>): IconType => {
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
          :key="volume.metadata.id"
          :aria-labelledby="`volume-${volume.metadata.id}-title`"
          class="whitespace-nowrap"
        >
          <TableCell>
            <div class="flex items-center gap-tight">
              <TIcon
                :icon="getFilesystemIcon(getEffectiveFilesystem(volume, volumeStatus))"
                class="size-4 shrink-0 text-content-emphasis"
              />

              <div>
                <div
                  :id="`volume-${volume.metadata.id}-title`"
                  class="max-w-40 truncate font-medium text-content-emphasis"
                  :title="volume.spec.partition_label || volume.spec.label || volume.metadata.id"
                >
                  {{ volume.spec.partition_label || volume.spec.label || volume.metadata.id }}
                </div>
                <div class="text-xs text-content-muted">
                  {{ volume.spec.dev_path }}
                </div>
              </div>
            </div>
          </TableCell>

          <TableCell class="font-medium">
            {{ getEffectiveFilesystem(volume, volumeStatus) }}
          </TableCell>

          <TableCell>
            <span
              class="inline-flex items-center gap-micro rounded p-micro"
              :class="getEncryptionClass(volumeStatus)"
            >
              <TIcon :icon="getEncryptionIcon(volumeStatus)" class="size-3" aria-hidden />
              <span class="text-xs/none">{{ isEncrypted(volumeStatus) }}</span>
            </span>
          </TableCell>

          <TableCell class="w-full max-w-20 truncate font-medium" :title="volume.spec.uuid">
            {{ volume.spec.uuid }}
          </TableCell>

          <TableCell class="font-medium">{{ prettyBytes(volume.spec.size ?? 0) }}</TableCell>
        </TableRow>
      </template>
    </TableRoot>
  </div>
</template>
