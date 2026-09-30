<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import {
  type ClusterKubernetesManifestsStatusSpecGroupStatus,
  KubernetesManifestGroupSpecMode,
} from '@/api/omni/specs/omni.pb'

export interface ClusterManifestsGroupNodeData {
  id: string
  group: ClusterKubernetesManifestsStatusSpecGroupStatus
  inSyncCount: number
  manifestCount: number
}

function modeName(mode?: KubernetesManifestGroupSpecMode) {
  switch (mode) {
    case KubernetesManifestGroupSpecMode.FULL:
      return 'Full'
    case KubernetesManifestGroupSpecMode.ONE_TIME:
      return 'One-Time'
    default:
      return 'Unknown'
  }
}
</script>

<script setup lang="ts">
import { Handle, type NodeProps, Position } from '@vue-flow/core'

const { dimensions, data } = defineProps<NodeProps<ClusterManifestsGroupNodeData>>()
</script>

<template>
  <div class="w-full rounded-lg border border-border-strong bg-surface-card shadow-lg/40">
    <div
      class="flex items-center gap-1 rounded-[7px] border border-border-accent bg-surface-card px-3"
      :style="{ height: `${dimensions.height}px` }"
    >
      <Handle id="right" type="source" :position="Position.Right" class="min-h-0! min-w-0!" />

      <span class="truncate text-sm font-medium text-content-emphasis">{{ id }}</span>
    </div>

    <div class="flex gap-4 px-4 py-2 text-xs">
      <div class="flex items-center gap-1">
        <span class="text-content-secondary">Mode:</span>
        <span class="text-content-emphasis">{{ modeName(data.group.mode) }}</span>
      </div>

      <div class="flex items-center gap-1">
        <span class="text-content-secondary">In sync:</span>
        <span class="text-content-emphasis">{{ data.inSyncCount }} / {{ data.manifestCount }}</span>
      </div>
    </div>
  </div>
</template>
