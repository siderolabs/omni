<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusLabelConnected } from '@/api/resources'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

const connected = (machine: Resource<ClusterMachineStatusSpec>): boolean => {
  if (machine.spec.stage === ClusterMachineStatusSpecStage.POWERING_ON) {
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
    default:
      return 'Unknown'
  }
}

const stageIcon = (machine: Resource<ClusterMachineStatusSpec>): IconType => {
  if (!connected(machine)) {
    return 'unknown'
  }

  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
    case ClusterMachineStatusSpecStage.INSTALLING:
    case ClusterMachineStatusSpecStage.UPGRADING:
    case ClusterMachineStatusSpecStage.CONFIGURING:
    case ClusterMachineStatusSpecStage.REBOOTING:
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
      return 'loading'
    case ClusterMachineStatusSpecStage.POWERING_ON:
      return 'power'
    case ClusterMachineStatusSpecStage.RUNNING:
      if (machine?.spec.ready) {
        return 'check-in-circle'
      } else {
        return 'error'
      }
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
      return 'loading'
    case ClusterMachineStatusSpecStage.DESTROYING:
      return 'delete'
    default:
      return 'unknown'
  }
}

const stageColor = (machine: Resource<ClusterMachineStatusSpec>): string => {
  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
    case ClusterMachineStatusSpecStage.INSTALLING:
    case ClusterMachineStatusSpecStage.UPGRADING:
    case ClusterMachineStatusSpecStage.CONFIGURING:
    case ClusterMachineStatusSpecStage.REBOOTING:
    case ClusterMachineStatusSpecStage.POWERING_ON:
      return 'var(--color-yellow-y1)'
    case ClusterMachineStatusSpecStage.RUNNING:
      if (machine?.spec.ready || !connected(machine)) {
        return 'var(--color-green-g1)'
      } else {
        return 'var(--color-red-r1)'
      }
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
    case ClusterMachineStatusSpecStage.DESTROYING:
      return 'var(--color-red-r1)'
    default:
      return 'var(--color-yellow-y1)'
  }
}

type Props = {
  machine: Resource<ClusterMachineStatusSpec>
}

defineProps<Props>()
</script>

<template>
  <div :style="'color: ' + stageColor(machine)">
    <Tooltip
      placement="bottom"
      :description="
        connected(machine) ? undefined : 'The machine is unreachable. The last known state is shown'
      "
    >
      <div
        class="flex gap-1"
        :class="'cluster-stage-box' + (connected(machine) ? '' : ' brightness-50')"
      >
        <TIcon :icon="stageIcon(machine)" class="h-4" />
        <div data-testid="cluster-machine-stage-name" class="flex-1 truncate">
          {{ stageName(machine) || '' }}
        </div>
      </div>
    </Tooltip>
  </div>
</template>

<style>
@reference "../../../index.css";

.cluster-stage-box {
  @apply flex items-center gap-1;
}
</style>
