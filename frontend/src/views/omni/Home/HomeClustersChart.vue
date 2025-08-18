<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { type ClusterStatusMetricsSpec, ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  EphemeralNamespace,
} from '@/api/resources'
import Card from '@/components/common/Card/Card.vue'
import RadialBar from '@/components/common/Charts/RadialBar.vue'
import { useWatch } from '@/components/common/Watch/useWatch'

const { items } = useWatch<ClusterStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: ClusterStatusMetricsType,
    id: ClusterStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const item = computed<Resource<ClusterStatusMetricsSpec> | undefined>(() => items.value[0])
</script>

<template>
  <Card>
    <RadialBar
      title="Clusters"
      show-hollow-total
      :items="[
        {
          label: 'Healthy',
          value:
            (item?.spec.phases?.[ClusterStatusSpecPhase.RUNNING] ?? 0) -
            (item?.spec.not_ready_count ?? 0),
        },
        { label: 'Unhealthy', value: item?.spec.not_ready_count ?? 0 },
        { label: 'Scaling Up', value: item?.spec.phases?.[ClusterStatusSpecPhase.SCALING_UP] ?? 0 },
        {
          label: 'Scaling Down',
          value: item?.spec.phases?.[ClusterStatusSpecPhase.SCALING_DOWN] ?? 0,
        },
        { label: 'Destroying', value: item?.spec.phases?.[ClusterStatusSpecPhase.DESTROYING] ?? 0 },
      ]"
    />
  </Card>
</template>
