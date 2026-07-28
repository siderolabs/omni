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
  <div class="w-full rounded-lg border border-naturals-n6 bg-naturals-n2 shadow-lg/40">
    <div
      class="flex items-center gap-1 rounded-[7px] border border-primary-p3 bg-naturals-n2 px-3"
      :style="{ height: `${dimensions.height}px` }"
    >
      <Handle id="right" type="source" :position="Position.Right" class="min-h-0! min-w-0!" />

      <span class="truncate text-sm font-medium text-naturals-n14">{{ id }}</span>
    </div>

    <div class="flex gap-4 px-4 py-2 text-xs">
      <div class="flex items-center gap-1">
        <span class="text-naturals-n11">Mode:</span>
        <span class="text-naturals-n14">{{ modeName(data.group.mode) }}</span>
      </div>

      <div class="flex items-center gap-1">
        <span class="text-naturals-n11">In sync:</span>
        <span class="text-naturals-n14">{{ data.inSyncCount }} / {{ data.manifestCount }}</span>
      </div>
    </div>
  </div>
</template>
