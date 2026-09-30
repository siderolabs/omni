<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources'
import Card from '@/components/Card/Card.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import HomeStatusSegmentedBar from '@/views/Home/HomeStatusSegmentedBar.vue'

const { data } = useResourceWatch<MachineStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const colors = {
  connected: 'var(--color-accent-default)',
  notConnected: 'var(--color-status-danger-default)',
  inCluster: 'var(--color-status-success-default)',
  freeMachine: 'var(--color-status-info-default)',
  pending: 'var(--color-status-warning-default)',
}

const counts = computed(() => {
  const spec = data.value?.spec

  const allocatedCount = spec?.allocated_machines_count ?? 0
  const connectedCount = spec?.connected_machines_count ?? 0
  const pendingCount = spec?.pending_machines_count ?? 0
  const registeredCount = spec?.registered_machines_count ?? 0

  return {
    totalCount: registeredCount + pendingCount,
    connectedCount,
    pendingCount,
    notConnectedCount: Math.max(registeredCount - connectedCount, 0),
    inClusterCount: allocatedCount,
    freeMachineCount: Math.max(registeredCount - allocatedCount, 0),
  }
})

const connectionItems = computed(() => [
  { label: 'Connected', value: counts.value.connectedCount, color: colors.connected },
  { label: 'Not Connected', value: counts.value.notConnectedCount, color: colors.notConnected },
])

const allocationItems = computed(() => [
  { label: 'In Cluster', value: counts.value.inClusterCount, color: colors.inCluster },
  { label: 'Free Machine', value: counts.value.freeMachineCount, color: colors.freeMachine },
  { label: 'Pending', value: counts.value.pendingCount, color: colors.pending },
])
</script>

<template>
  <Card class="p-4">
    <HomeStatusSegmentedBar
      title="Machines"
      :total="counts.totalCount"
      :bars="[
        { label: 'Connection', segments: connectionItems },
        { label: 'Allocation', segments: allocationItems },
      ]"
    />
  </Card>
</template>
