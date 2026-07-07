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
      color: 'var(--color-primary-p3)',
    },
    { label: 'Unhealthy', value: notReadyCount, color: 'var(--color-red-r1)' },
    {
      label: 'Scaling Up',
      value: spec?.phases?.[ClusterStatusSpecPhase.SCALING_UP] ?? 0,
      color: 'var(--color-green-g1)',
    },
    {
      label: 'Scaling Down',
      value: spec?.phases?.[ClusterStatusSpecPhase.SCALING_DOWN] ?? 0,
      color: 'var(--color-blue-b1)',
    },
    {
      label: 'Destroying',
      value: spec?.phases?.[ClusterStatusSpecPhase.DESTROYING] ?? 0,
      color: 'var(--color-yellow-y1)',
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
