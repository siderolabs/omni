<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { MachineOperationStatusSpec } from '@/api/omni/specs/omni.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelConnected } from '@/api/resources'
import StatusPill from '@/components/Status/StatusPill.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'
import ClusterMachineOperationDescription from '@/views/ClusterMachines/ClusterMachineOperationDescription.vue'

const { machine, operation } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
  operation?: MachineOperationStatusSpec
}>()

defineOptions({ inheritAttrs: false })

const { status } = useDerivedMachineStage(
  () => machine.spec,
  () => operation,
)

const isConnected = computed(
  () =>
    machine.spec.snapshot?.power_stage ===
      MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERING_ON ||
    machine.spec.snapshot?.power_stage ===
      MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERED_OFF ||
    machine.metadata.labels?.[MachineStatusLabelConnected] === '',
)

const tone = computed(() =>
  machine.spec.tearing_down || !isConnected.value || !status.value ? 'info' : status.value.tone,
)

const glyph = computed(() => {
  if (machine.spec.tearing_down) return 'progress'
  if (!isConnected.value) return 'unknown'

  return status.value?.glyph ?? 'unknown'
})

const shownOperation = computed(() =>
  machine.spec.tearing_down || !isConnected.value ? undefined : status.value?.operation,
)
</script>

<template>
  <Tooltip v-if="status" placement="bottom">
    <StatusPill v-bind="$attrs" :tone :glyph>
      {{ status.name }}
    </StatusPill>

    <template v-if="shownOperation" #description>
      <ClusterMachineOperationDescription :operation="shownOperation" />
    </template>
  </Tooltip>
</template>
