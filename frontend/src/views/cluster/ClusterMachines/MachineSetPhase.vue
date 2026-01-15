<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import type { Resource } from '@/api/grpc'
import type { MachineSetStatusSpec } from '@/api/omni/specs/omni.pb'
import { MachineSetPhase } from '@/api/omni/specs/omni.pb'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

const phaseName = (machineset: Resource<MachineSetStatusSpec>): string => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
      return 'Scaling Up'
    case MachineSetPhase.ScalingDown:
      return 'Scaling Down'
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return 'Running'
      } else {
        return 'Not Ready'
      }
    case MachineSetPhase.Destroying:
      return 'Destroying'
    case MachineSetPhase.Failed:
      return 'Failed'
    case MachineSetPhase.Reconfiguring:
      return 'Reconfiguring'
    default:
      return 'Unknown'
  }
}

const phaseIcon = (machineset: Resource<MachineSetStatusSpec>): IconType => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
    case MachineSetPhase.ScalingDown:
    case MachineSetPhase.Reconfiguring:
      return 'loading'
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return 'check-in-circle'
      } else {
        return 'error'
      }
    case MachineSetPhase.Destroying:
      return 'delete'
    case MachineSetPhase.Failed:
      return 'error'
    default:
      return 'unknown'
  }
}

const phaseColor = (machineset): string => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
    case MachineSetPhase.ScalingDown:
    case MachineSetPhase.Reconfiguring:
      return 'var(--color-yellow-y1)'
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return 'var(--color-green-g1)'
      } else {
        return 'var(--color-red-r1)'
      }
    case MachineSetPhase.Failed:
    case MachineSetPhase.Destroying:
      return 'var(--color-red-r1)'
    default:
      return 'var(--color-yellow-y1)'
  }
}

type Props = {
  item: Resource<MachineSetStatusSpec>
}

defineProps<Props>()
</script>

<template>
  <div class="flex gap-2">
    <div :style="'color: ' + phaseColor(item)" class="cluster-phase-box">
      <TIcon :icon="phaseIcon(item)" class="h-4" />
      <div data-testid="machine-set-phase-name">{{ phaseName(item) || '' }}</div>
    </div>
    <div v-if="item.spec.locked_updates" class="flex items-center gap-1 text-sky-400">
      <TIcon icon="time" class="h-4" />
      {{ pluralize('Pending Config Update', item.spec.locked_updates, true) }}
    </div>
  </div>
</template>

<style>
@reference "../../../index.css";

.cluster-phase-box {
  @apply flex items-center gap-1;
}
</style>
