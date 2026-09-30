<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'

type Props = {
  cluster?: Resource<ClusterStatusSpec>
}

defineProps<Props>()

const phaseName = (cluster?: Resource<ClusterStatusSpec>): string => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
      return 'Scaling Up'
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return 'Scaling Down'
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return 'Running'
      } else {
        return 'Not Ready'
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return 'Destroying'
    default:
      return 'Unknown'
  }
}

const phaseStatus = (cluster?: Resource<ClusterStatusSpec>): StatusAppearance => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return { tone: 'warning', glyph: 'progress' }
    case ClusterStatusSpecPhase.DESTROYING:
      return { tone: 'info', glyph: 'progress' }
    case ClusterStatusSpecPhase.RUNNING:
      return cluster?.spec.ready
        ? { tone: 'success', glyph: 'success' }
        : { tone: 'danger', glyph: 'danger' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}
</script>

<template>
  <StatusPill v-bind="phaseStatus(cluster)">
    {{ phaseName(cluster) }}
  </StatusPill>
</template>
