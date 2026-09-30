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
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'

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

function manifestPhaseStatus(
  phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
): StatusAppearance {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return { tone: 'success', glyph: 'success' }
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return { tone: 'warning', glyph: 'progress' }
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return { tone: 'info', glyph: 'progress' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}
</script>

<script setup lang="ts">
const { phase = ClusterKubernetesManifestsStatusSpecManifestStatusPhase.UNKNOWN } = defineProps<{
  phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase
}>()
</script>

<template>
  <StatusPill v-bind="manifestPhaseStatus(phase)">
    {{ manifestPhaseName(phase) }}
  </StatusPill>
</template>
