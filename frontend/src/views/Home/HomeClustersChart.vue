<!--
Copyright (c) 2026 Sidero Labs, Inc.

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
import Card from '@/components/Card/Card.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import HomeStatusSegmentedBar from '@/views/Home/HomeStatusSegmentedBar.vue'

const { data } = useResourceWatch<ClusterStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: ClusterStatusMetricsType,
    id: ClusterStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const items = computed(() => {
  const spec = data.value?.spec

  const notReadyCount = spec?.not_ready_count ?? 0
  const runningCount = spec?.phases?.[ClusterStatusSpecPhase.RUNNING] ?? 0

  return [
    {
      label: 'Healthy',
      value: Math.max(runningCount - notReadyCount, 0),
      color: 'var(--color-status-success-chart)',
    },
    { label: 'Unhealthy', value: notReadyCount, color: 'var(--color-status-danger-chart)' },
    {
      // Up and down share a colour in the status pill, so they share a segment here.
      label: 'Scaling',
      value:
        (spec?.phases?.[ClusterStatusSpecPhase.SCALING_UP] ?? 0) +
        (spec?.phases?.[ClusterStatusSpecPhase.SCALING_DOWN] ?? 0),
      color: 'var(--color-status-warning-chart)',
    },
    {
      label: 'Destroying',
      value: spec?.phases?.[ClusterStatusSpecPhase.DESTROYING] ?? 0,
      color: 'var(--color-status-info-chart)',
    },
  ]
})
</script>

<template>
  <Card class="p-4">
    <HomeStatusSegmentedBar
      title="Clusters"
      :total="items.reduce((sum, item) => sum + item.value, 0)"
      :bars="[{ segments: items }]"
    />
  </Card>
</template>
