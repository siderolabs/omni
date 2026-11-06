// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { formatDistanceStrict, parseISO } from 'date-fns'
import capitalize from 'lodash/capitalize'
import startCase from 'lodash/startCase'

import type { ClusterMachineStatusSpecOperation } from '@/api/omni/specs/omni.pb'
import { MachineOperationPhase } from '@/api/omni/specs/omni.pb'

export function machineOperationPhaseName(phase?: MachineOperationPhase) {
  if (!phase) {
    return ''
  }

  const name: string | undefined = MachineOperationPhase[phase]

  return startCase((name ?? '').toLowerCase())
}

export function machineOperationDescription(
  operation: ClusterMachineStatusSpecOperation,
  now: Date,
) {
  const parts: string[] = []

  if ((operation.config_try_attempt ?? 0) > 1) {
    parts.push(`Attempt ${operation.config_try_attempt}`)
  }

  if (operation.message) {
    parts.push(operation.message)
  }

  if (operation.phase_started_at) {
    parts.push(
      `started ${formatDistanceStrict(parseISO(operation.phase_started_at), now, { addSuffix: true })}`,
    )
  }

  return parts.join(', ')
}

export function machineOperationErrorTitle(operation?: ClusterMachineStatusSpecOperation) {
  const phase = machineOperationPhaseName(operation?.failed_phase)

  return phase ? `${phase} Failed` : 'Configuration Error'
}

export function machineOperationError(
  lastConfigError?: string,
  operation?: ClusterMachineStatusSpecOperation,
) {
  const operationError = operation?.last_error?.trim()
  const configError = lastConfigError?.trim()

  if (!operationError) {
    return configError ?? ''
  }

  if (!configError) {
    return operationError
  }

  return `${operationError}\nConfiguration error: ${configError}`
}

export function machineOperationErrorDescription(
  lastConfigError?: string,
  operation?: ClusterMachineStatusSpecOperation,
) {
  const title = capitalize(machineOperationErrorTitle(operation))

  return `${title}: ${machineOperationError(lastConfigError, operation)}`
}
