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

export interface ClusterManifestsManifestNodeData {
  manifest: ClusterKubernetesManifestsStatusSpecManifestStatus
}

function manifestPhaseName(phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase) {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return 'Pending'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return 'Applied'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return 'Deleting'
    default:
      return 'Unknown'
  }
}

function manifestPhaseClass(phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase) {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return 'text-green-g1'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return 'text-yellow-y1'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return 'text-red-r1'
    default:
      return 'text-naturals-n9'
  }
}
</script>

<script setup lang="ts">
const { phase = ClusterKubernetesManifestsStatusSpecManifestStatusPhase.UNKNOWN } = defineProps<{
  phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase
}>()
</script>

<template>
  <span
    class="w-max rounded bg-current/20 px-1.5 py-0.5 text-[0.625rem]"
    :class="manifestPhaseClass(phase)"
  >
    {{ manifestPhaseName(phase) }}
  </span>
</template>
