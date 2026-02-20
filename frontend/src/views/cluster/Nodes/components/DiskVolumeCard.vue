<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import TIcon, { type IconType } from '@/components/common/Icon/TIcon.vue'
import type {
  TalosDiscoveredVolumeSpec,
  TalosMountStatusSpec,
} from '@/views/cluster/Nodes/NodeDisks.vue'

defineProps<{
  volume: Resource<TalosDiscoveredVolumeSpec>
  mount?: Resource<TalosMountStatusSpec>
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

const isEncrypted = (item?: Resource<TalosMountStatusSpec>) => {
  if (item?.spec.encrypted === undefined) {
    return Encryption.Unknown
  }

  return item.spec.encrypted ? Encryption.Enabled : Encryption.Disabled
}

const getEncryptionClass = (item?: Resource<TalosMountStatusSpec>) => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'text-red-r1'
    case Encryption.Enabled:
      return 'text-green-g1'
    case Encryption.Unknown:
      return 'text-naturals-n11'
  }
}

const getEncryptionIcon = (item?: Resource<TalosMountStatusSpec>): IconType => {
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
  <section
    :aria-labelledby="`volume-${volume.metadata.id}-title`"
    class="rounded border border-naturals-n7 bg-naturals-n2 p-3"
  >
    <div class="mb-3 flex items-start justify-between">
      <div class="flex items-center gap-2">
        <TIcon :icon="getFilesystemIcon(volume.spec.name)" class="size-4 text-naturals-n13" />
        <div>
          <div :id="`volume-${volume.metadata.id}-title`" class="font-medium text-naturals-n14">
            {{ volume.spec.partition_label || volume.spec.label || volume.metadata.id }}
          </div>
          <div class="text-xs text-naturals-n11">
            {{ volume.spec.dev_path }}
          </div>
        </div>
      </div>
      <div class="text-right">
        <div class="text-sm font-medium text-naturals-n14">
          {{ volume.spec.pretty_size }}
        </div>
      </div>
    </div>

    <dl class="space-y-2 text-sm">
      <div class="flex items-center justify-between">
        <dt class="text-naturals-n11">Filesystem:</dt>
        <dd class="font-mono text-naturals-n13">
          {{ volume.spec.name || 'N/A' }}
        </dd>
      </div>

      <div v-if="volume.spec.uuid" class="flex items-center justify-between">
        <dt class="text-naturals-n11">UUID:</dt>
        <dd class="max-w-50 truncate font-mono text-xs text-naturals-n13" :title="volume.spec.uuid">
          {{ volume.spec.uuid }}
        </dd>
      </div>

      <div class="flex items-center justify-between">
        <dt class="text-naturals-n11">Encryption:</dt>
        <dd
          class="inline-flex items-center gap-1 rounded bg-naturals-n4 px-2 py-0.5"
          :class="getEncryptionClass(mount)"
        >
          <TIcon
            :icon="getEncryptionIcon(mount)"
            :class="isEncrypted(mount) === Encryption.Unknown ? 'size-3.5' : 'size-3'"
            aria-hidden
          />
          <span class="text-xs">{{ isEncrypted(mount) }}</span>
        </dd>
      </div>
    </dl>
  </section>
</template>
