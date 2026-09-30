<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelConnected } from '@/api/resources'
import StatusPill from '@/components/Status/StatusPill.vue'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'

const { machine } = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
}>()

const { status } = useDerivedMachineStage(() => machine.spec.snapshot)

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
</script>

<template>
  <StatusPill v-if="status" :tone :glyph>
    {{ status.name }}
  </StatusPill>
</template>
