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
    class="flex size-full items-center gap-tight rounded-md border border-border-strong bg-surface-card px-snug py-tight shadow-lg/40"
  >
    <Handle id="left" type="target" :position="Position.Left" class="min-h-0! min-w-0!" />

    <div
      class="size-2 rounded-xs border border-current"
      :class="{
        'bg-current text-status-success-text': isApplied,
        'text-status-warning-text': isPending,
        'text-status-danger-text': !isApplied && !isPending,
      }"
    ></div>

    <div class="flex min-w-0 grow flex-col gap-micro leading-tight">
      <span class="truncate text-[0.6875rem] font-medium text-content-default">
        {{ data.manifest.name }}
      </span>

      <ClusterManifestPhase class="shrink-0" :phase="data.manifest.phase" />

      <div v-if="data.manifest.kind" class="flex items-center gap-micro truncate text-[0.5625rem]">
        <span class="text-content-muted">Kind:</span>
        <span class="text-content-default">{{ data.manifest.kind }}</span>
      </div>

      <div
        v-if="data.manifest.namespace"
        class="flex items-center gap-micro truncate text-[0.5625rem]"
      >
        <span class="text-content-muted">Namespace:</span>
        <span class="text-content-default">{{ data.manifest.namespace }}</span>
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
