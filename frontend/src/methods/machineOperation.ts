// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { formatDistanceStrict, parseISO } from 'date-fns'
import startCase from 'lodash/startCase'

import type { MachineOperationStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusSpecStage,
  MachineOperationStatusSpecStage,
} from '@/api/omni/specs/omni.pb'

export function machineOperationStageName(operation?: MachineOperationStatusSpec) {
  if (!operation?.stage) {
    return ''
  }

  const name: string | undefined = MachineOperationStatusSpecStage[operation.stage]

  return startCase((name ?? '').toLowerCase())
}

export function machineOperationDescription(
  operation: MachineOperationStatusSpec | undefined,
  now: Date,
) {
  if (!operation?.stage_started_at) {
    return ''
  }

  return `Started ${formatDistanceStrict(parseISO(operation.stage_started_at), now, { addSuffix: true })}`
}

export function machineOperationProgress(operation?: MachineOperationStatusSpec) {
  return operation?.message || machineOperationStageName(operation)
}

// The stages of a machine which is going away or powered off tell more than its operation.
export function clusterMachineShowsOperation(stage?: ClusterMachineStatusSpecStage) {
  switch (stage) {
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
    case ClusterMachineStatusSpecStage.DESTROYING:
    case ClusterMachineStatusSpecStage.POWERING_ON:
    case ClusterMachineStatusSpecStage.POWERED_OFF:
      return false
    default:
      return true
  }
}
