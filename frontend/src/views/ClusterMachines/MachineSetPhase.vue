<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import type { Resource } from '@/api/grpc'
import type { MachineSetStatusSpec } from '@/api/omni/specs/omni.pb'
import { MachineSetPhase } from '@/api/omni/specs/omni.pb'
import TIcon from '@/components/Icon/TIcon.vue'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'

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
    case MachineSetPhase.Upgrading:
      return 'Upgrading'
    default:
      return 'Unknown'
  }
}

const phaseStatus = (machineset?: Resource<MachineSetStatusSpec>): StatusAppearance => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.Upgrading:
    case MachineSetPhase.ScalingUp:
    case MachineSetPhase.ScalingDown:
    case MachineSetPhase.Reconfiguring:
      return { tone: 'warning', glyph: 'progress' }
    case MachineSetPhase.Destroying:
      return { tone: 'info', glyph: 'progress' }
    case MachineSetPhase.Running:
      return machineset?.spec.ready
        ? { tone: 'success', glyph: 'success' }
        : { tone: 'danger', glyph: 'danger' }
    case MachineSetPhase.Failed:
      return { tone: 'danger', glyph: 'danger' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}

type Props = {
  item: Resource<MachineSetStatusSpec>
}

defineProps<Props>()
</script>

<template>
  <div class="flex items-center gap-tight">
    <StatusPill v-bind="phaseStatus(item)" data-testid="machine-set-phase-name">
      {{ phaseName(item) || '' }}
    </StatusPill>
    <div v-if="item.spec.locked_updates" class="flex items-center gap-micro text-status-info-text">
      <TIcon icon="time" class="size-4 shrink-0" />
      {{ pluralize('Pending Config Update', item.spec.locked_updates, true) }}
    </div>
  </div>
</template>
