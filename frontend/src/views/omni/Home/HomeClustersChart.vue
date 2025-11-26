<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type ClusterStatusMetricsSpec, ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  EphemeralNamespace,
} from '@/api/resources'
import Card from '@/components/common/Card/Card.vue'
import RadialBar from '@/components/common/Charts/RadialBar.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { data } = useResourceWatch<ClusterStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: ClusterStatusMetricsType,
    id: ClusterStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const counts = computed(() => {
  const spec = data.value?.spec

  const notReadyCount = spec?.not_ready_count ?? 0
  const destroyingCount = spec?.phases?.[ClusterStatusSpecPhase.DESTROYING] ?? 0
  const runningCount = spec?.phases?.[ClusterStatusSpecPhase.RUNNING] ?? 0
  const scalingDownCount = spec?.phases?.[ClusterStatusSpecPhase.SCALING_DOWN] ?? 0
  const scalingUpCount = spec?.phases?.[ClusterStatusSpecPhase.SCALING_UP] ?? 0

  return {
    healthyCount: runningCount - notReadyCount,
    unhealthyCount: notReadyCount,
    scalingUpCount,
    scalingDownCount,
    destroyingCount,
  }
})
</script>

<template>
  <Card class="p-4">
    <RadialBar
      title="Clusters"
      show-hollow-total
      :items="[
        { label: 'Healthy', value: counts.healthyCount },
        { label: 'Unhealthy', value: counts.unhealthyCount },
        { label: 'Scaling Up', value: counts.scalingUpCount },
        { label: 'Scaling Down', value: counts.scalingDownCount },
        { label: 'Destroying', value: counts.destroyingCount },
      ]"
    />
  </Card>
</template>
