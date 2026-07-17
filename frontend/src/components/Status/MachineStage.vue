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
import TIcon from '@/components/Icon/TIcon.vue'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'
import { cn } from '@/methods/utils'

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
</script>

<template>
  <span
    v-if="status"
    :class="
      cn(
        'inline-flex items-center gap-1 text-xs',
        status.class,
        { 'brightness-50': machine.spec.tearing_down || !isConnected },
        $attrs.class,
      )
    "
  >
    <TIcon :icon="isConnected ? status.icon : 'unknown'" class="size-4" aria-hidden="true" />
    {{ status.name }}
  </span>
</template>
