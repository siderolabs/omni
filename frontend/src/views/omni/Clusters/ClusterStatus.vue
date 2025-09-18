<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

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

const phaseIcon = (cluster?: Resource<ClusterStatusSpec>): IconType => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return 'loading'
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return 'check-in-circle'
      } else {
        return 'error'
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return 'delete'
    default:
      return 'unknown'
  }
}

const phaseColor = (cluster?: Resource<ClusterStatusSpec>): string => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return 'var(--color-yellow-y1)'
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return 'var(--color-green-g1)'
      } else {
        return 'var(--color-red-r1)'
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return 'var(--color-red-r1)'
    default:
      return 'var(--color-yellow-y1)'
  }
}
</script>

<template>
  <div :style="'color: ' + phaseColor(cluster)" class="flex items-center gap-1">
    <TIcon :icon="phaseIcon(cluster)" class="h-4" aria-hidden="true" />
    <span class="contents max-sm:sr-only">{{ phaseName(cluster) }}</span>
  </div>
</template>
