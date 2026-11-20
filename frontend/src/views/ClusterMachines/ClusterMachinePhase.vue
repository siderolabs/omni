<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec, MachineOperationStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelConnected } from '@/api/resources'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { clusterMachineShowsOperation, machineOperationStageName } from '@/methods/machineOperation'

import ClusterMachineOperationDescription from './ClusterMachineOperationDescription.vue'

const connected = (machine: Resource<ClusterMachineStatusSpec>): boolean => {
  if (
    machine.spec.stage === ClusterMachineStatusSpecStage.POWERING_ON ||
    machine.spec.stage === ClusterMachineStatusSpecStage.POWERED_OFF
  ) {
    return true
  }

  return machine?.metadata.labels?.[MachineStatusLabelConnected] === ''
}

const stageName = (machine: Resource<ClusterMachineStatusSpec>): string => {
  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
      return 'Booting'
    case ClusterMachineStatusSpecStage.INSTALLING:
      return 'Installing'
    case ClusterMachineStatusSpecStage.UPGRADING:
      return 'Upgrading'
    case ClusterMachineStatusSpecStage.CONFIGURING:
      return 'Configuring'
    case ClusterMachineStatusSpecStage.RUNNING:
      if (machine?.spec.ready || !connected(machine)) {
        return 'Running'
      } else {
        return 'Not Ready'
      }
    case ClusterMachineStatusSpecStage.REBOOTING:
      return 'Rebooting'
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
      return 'Shutting Down'
    case ClusterMachineStatusSpecStage.DESTROYING:
      return 'Destroying'
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
      return 'Preparing to Destroy'
    case ClusterMachineStatusSpecStage.POWERING_ON:
      return 'Powering On'
    case ClusterMachineStatusSpecStage.POWERED_OFF:
      return 'Powered Off'
    default:
      return 'Unknown'
  }
}

const stageStatus = (machine: Resource<ClusterMachineStatusSpec>): StatusAppearance => {
  if (!connected(machine)) {
    return { tone: 'info', glyph: 'unknown' }
  }

  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
    case ClusterMachineStatusSpecStage.INSTALLING:
    case ClusterMachineStatusSpecStage.UPGRADING:
    case ClusterMachineStatusSpecStage.CONFIGURING:
    case ClusterMachineStatusSpecStage.REBOOTING:
    case ClusterMachineStatusSpecStage.POWERING_ON:
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
    case ClusterMachineStatusSpecStage.DESTROYING:
      return { tone: 'info', glyph: 'progress' }
    case ClusterMachineStatusSpecStage.POWERED_OFF:
      return { tone: 'info', glyph: 'neutral' }
    case ClusterMachineStatusSpecStage.RUNNING:
      return machine?.spec.ready
        ? { tone: 'success', glyph: 'success' }
        : { tone: 'danger', glyph: 'danger' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}

type Phase = {
  name: string
  appearance: StatusAppearance
  unreachable?: boolean
  operation?: MachineOperationStatusSpec
}

const { machine, operation } = defineProps<{
  machine: Resource<ClusterMachineStatusSpec>
  operation?: MachineOperationStatusSpec
}>()

const phase = computed((): Phase => {
  const operationName = clusterMachineShowsOperation(machine.spec.stage)
    ? machineOperationStageName(operation)
    : ''

  if (!connected(machine)) {
    return {
      name: operationName || stageName(machine),
      appearance: stageStatus(machine),
      unreachable: true,
    }
  }

  if (operationName) {
    return {
      name: operationName,
      appearance: { tone: 'warning', glyph: 'progress' },
      operation,
    }
  }

  return { name: stageName(machine), appearance: stageStatus(machine) }
})
</script>

<template>
  <div>
    <Tooltip placement="bottom">
      <StatusPill v-bind="phase.appearance">
        <span data-testid="cluster-machine-stage-name" class="truncate">
          {{ phase.name }}
        </span>
      </StatusPill>

      <template v-if="phase.unreachable || phase.operation" #description>
        <p v-if="phase.unreachable" class="whitespace-pre">
          The machine is unreachable. The last known state is shown
        </p>

        <ClusterMachineOperationDescription
          v-else-if="phase.operation"
          :operation="phase.operation"
        />
      </template>
    </Tooltip>
  </div>
</template>
