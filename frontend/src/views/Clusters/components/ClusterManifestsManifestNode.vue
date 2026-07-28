<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import {
  type ClusterKubernetesManifestsStatusSpecManifestStatus,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
} from '@/api/omni/specs/omni.pb'
import IconButton from '@/components/Button/IconButton.vue'
import ClusterManifestPhase from '@/views/Clusters/components/ClusterManifestPhase.vue'

export interface ClusterManifestsManifestNodeData {
  groupId: string
  manifest: ClusterKubernetesManifestsStatusSpecManifestStatus
}
</script>

<script setup lang="ts">
import { Handle, type NodeProps, Position } from '@vue-flow/core'
import { computed } from 'vue'

const { data } = defineProps<NodeProps<ClusterManifestsManifestNodeData>>()

defineEmits<{
  manifestClick: [string, ClusterKubernetesManifestsStatusSpecManifestStatus]
}>()

const isApplied = computed(
  () => data.manifest.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
)

const isPending = computed(
  () => data.manifest.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING,
)
</script>

<template>
  <div
    class="flex size-full items-center gap-2 rounded-md border border-naturals-n6 bg-naturals-n2 px-2.5 py-2 shadow-lg/40"
  >
    <Handle id="left" type="target" :position="Position.Left" class="min-h-0! min-w-0!" />

    <div
      class="size-2 rounded-xs border border-current"
      :class="{
        'bg-current text-green-g1': isApplied,
        'text-yellow-y1': isPending,
        'text-red-r1': !isApplied && !isPending,
      }"
    ></div>

    <div class="flex min-w-0 grow flex-col gap-1 leading-tight">
      <span class="truncate text-[0.6875rem] font-medium text-naturals-n12">
        {{ data.manifest.name }}
      </span>

      <ClusterManifestPhase class="shrink-0" :phase="data.manifest.phase" />

      <div v-if="data.manifest.kind" class="flex items-center gap-0.5 truncate text-[0.5625rem]">
        <span class="text-naturals-n10">Kind:</span>
        <span class="text-naturals-n12">{{ data.manifest.kind }}</span>
      </div>

      <div
        v-if="data.manifest.namespace"
        class="flex items-center gap-0.5 truncate text-[0.5625rem]"
      >
        <span class="text-naturals-n10">Namespace:</span>
        <span class="text-naturals-n12">{{ data.manifest.namespace }}</span>
      </div>
    </div>

    <IconButton
      icon="eye"
      aria-label="view manifest"
      class="pointer-events-auto"
      @click="$emit('manifestClick', data.groupId, data.manifest)"
    />
  </div>
</template>
