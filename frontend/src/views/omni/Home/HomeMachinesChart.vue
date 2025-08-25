<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources'
import Card from '@/components/common/Card/Card.vue'
import RadialBar from '@/components/common/Charts/RadialBar.vue'
import { useWatch } from '@/components/common/Watch/useWatch'

const { items } = useWatch<MachineStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const item = computed<Resource<MachineStatusMetricsSpec> | undefined>(() => items.value[0])
</script>

<template>
  <Card>
    <RadialBar
      title="Machines"
      show-hollow-total
      :items="[
        { label: 'Connected', value: item?.spec.connected_machines_count ?? 0 },
        {
          label: 'Not Connected',
          value:
            (item?.spec.registered_machines_count ?? 0) -
            (item?.spec.connected_machines_count ?? 0),
        },
        { label: 'In Cluster', value: item?.spec.allocated_machines_count ?? 0 },
        {
          label: 'Free Machine',
          value:
            (item?.spec.registered_machines_count ?? 0) -
            (item?.spec.allocated_machines_count ?? 0),
        },
      ]"
    />
  </Card>
</template>
