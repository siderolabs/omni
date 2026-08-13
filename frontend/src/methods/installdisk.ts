// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { MachineInstallDiskStatusSpec } from '@/api/omni/specs/omni.pb'

export type InstallDiskSelectItem = {
  label: string
  value: string
  tooltip?: string
  disabled?: boolean
}

// installDiskSelectItems builds the dropdown entries from the per-disk evaluations: plain
// entries for the candidates, labeled ones for the disks only an explicit selection can
// target. The backend evaluates and orders the disks, no selection rules live here. The
// ineligible disks are not rendered for the time being.
export function installDiskSelectItems({ disks = [] }: MachineInstallDiskStatusSpec = {}) {
  return disks
    .filter((d): d is typeof d & { dev_path: string } => !!d.selectable && !!d.dev_path)
    .map<InstallDiskSelectItem>((disk) => ({
      label: disk.members?.length
        ? `${disk.dev_path} (on ${disk.members.join(', ')})`
        : disk.dev_path,
      value: disk.dev_path,
      tooltip: disk.skip_reason || undefined,
    }))
}

// installDiskFallbackItem builds the entry for a disk referenced by the selection or the
// resolution but not offered. An unreported disk stays pickable, so an existing selection
// keeps its label. A reported disk is assumed non-selectable (the caller checked it is not
// offered) and is shown disabled with its skip reason.
export function installDiskFallbackItem(
  devPath: string,
  { disks }: MachineInstallDiskStatusSpec = {},
): InstallDiskSelectItem {
  const reported = disks?.find((disk) => disk.dev_path === devPath)

  return {
    label: devPath,
    value: devPath,
    tooltip: reported?.skip_reason || undefined,
    disabled: !!reported,
  }
}
