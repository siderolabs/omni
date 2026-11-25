// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import {
  type DryRunStatusSpec,
  DryRunStatusSpecNotCheckedReason,
  DryRunStatusSpecOutcome,
} from '@/api/omni/specs/omni.pb'

function notCheckedReason(reason?: DryRunStatusSpecNotCheckedReason) {
  switch (reason) {
    case DryRunStatusSpecNotCheckedReason.DISCONNECTED:
      return 'disconnected'
    case DryRunStatusSpecNotCheckedReason.TRY_IN_FLIGHT:
      return 'config try in flight'
    case DryRunStatusSpecNotCheckedReason.MAINTENANCE:
      return 'maintenance mode'
    case DryRunStatusSpecNotCheckedReason.NOT_RUNNING:
      return 'not running'
    case DryRunStatusSpecNotCheckedReason.CHECK_ERROR:
      return 'check failed'
    default:
      return 'unknown'
  }
}

export type DryRunFailure = {
  id: string
  name: string
  error?: string
}

export function dryRunResult(dryRun: DryRunStatusSpec) {
  switch (dryRun.outcome) {
    case DryRunStatusSpecOutcome.PASSED:
      return dryRun.reboot_required ? 'would reboot' : 'would apply'
    case DryRunStatusSpecOutcome.FAILED:
      return 'would fail'
    case DryRunStatusSpecOutcome.NOT_CHECKED:
      return 'not checked'
  }
}

export function dryRunSummary(dryRun: DryRunStatusSpec) {
  switch (dryRun.outcome) {
    case DryRunStatusSpecOutcome.FAILED:
      return 'Pending config would fail'
    case DryRunStatusSpecOutcome.NOT_CHECKED:
      return `Not checked: ${notCheckedReason(dryRun.not_checked_reason)}`
    default:
      return 'Dry run result unknown'
  }
}
