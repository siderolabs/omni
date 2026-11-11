// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { DiskSpec, SMARTStatusSpec } from '@/api/talos/block.pb'
import type {
  LVMLogicalVolumeStatusSpec,
  LVMVolumeGroupStatusSpec,
  MDArrayStatusSpec,
} from '@/api/talos/storage.pb'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import type { AlertType } from '@/components/TAlert.vue'

import type { DiskDetail } from './diskDetails'

/** A device's health, as one state to show. */
export interface DiskHealth extends StatusAppearance {
  label: string
  /** Why the device is in this state, when it isn't healthy. */
  message?: string
}

const healthy: DiskHealth = { label: 'Healthy', tone: 'success', glyph: 'success' }

// What MD is doing while it syncs, from sysfs sync_action, and whether it means
// the array lost redundancy. A check or repair is routine scrubbing.
const syncAppearances: Record<string, Pick<DiskHealth, 'label' | 'tone'>> = {
  check: { label: 'Checking', tone: 'info' },
  repair: { label: 'Repairing', tone: 'info' },
  resync: { label: 'Resyncing', tone: 'warning' },
  recover: { label: 'Rebuilding', tone: 'warning' },
  reshape: { label: 'Reshaping', tone: 'warning' },
  frozen: { label: 'Sync frozen', tone: 'warning' },
}

/** Returns the health of an MD array that Talos manages. */
export function getMDArrayHealth(array: MDArrayStatusSpec): DiskHealth {
  const { status, error, syncAction = '', raidDevices = 0, members = [] } = array

  if (status === 'error')
    return { label: 'Failed', tone: 'danger', glyph: 'danger', message: error }
  if (status === 'waiting')
    return { label: 'Waiting', tone: 'info', glyph: 'progress', message: error }

  // raidDevices is the number of slots the array is configured with. Members
  // include spares, so this can miss a degraded array that has a spare, but
  // never reports a healthy one as degraded.
  if (members.length < raidDevices) {
    return {
      label: 'Degraded',
      tone: 'warning',
      glyph: 'warning',
      message: `${members.length} of ${raidDevices} members are present.`,
    }
  }

  // Talos reports any sync other than idle as rebuilding, a routine check too.
  if (status === 'rebuilding') {
    const sync = syncAppearances[syncAction] ?? { label: 'Rebuilding', tone: 'warning' }

    return { ...sync, glyph: 'progress', message: error }
  }

  return status === 'ready'
    ? healthy
    : { label: status || 'Unknown', tone: 'info', glyph: 'unknown' }
}

/** The alert type to show a health problem with. */
export function getHealthAlertType(health: DiskHealth): AlertType {
  switch (health.tone) {
    case 'danger':
      return 'error'
    case 'warning':
      return 'warn'
    default:
      return 'info'
  }
}

/**
 * Returns the MD arrays Talos manages that have no device yet, such as an array
 * Talos failed to create. Their status has no device to be shown on.
 */
export function getArraysWithoutDevice(
  arrays: Resource<MDArrayStatusSpec>[],
  disks: Resource<DiskSpec>[],
): Resource<MDArrayStatusSpec>[] {
  return arrays.filter(
    (a) => !a.spec.device || !disks.some((d) => d.spec.symlinks?.includes(a.spec.device!)),
  )
}

// A volume group missing physical volumes, or undefined when it has them all.
const getVolumeGroupProblem = (group: LVMVolumeGroupStatusSpec): DiskHealth | undefined => {
  const missing = Number(group.missingPVCount || 0)

  if (!missing && !group.partial) return

  return {
    label: 'Degraded',
    tone: 'warning',
    glyph: 'warning',
    message: `Volume group ${group.name} is missing ${missing || 'some'} physical volume${missing === 1 ? '' : 's'}.`,
  }
}

/** Returns the health of a logical volume, and of the volume group it is in. */
export function getLogicalVolumeHealth(
  volume: LVMLogicalVolumeStatusSpec,
  group?: LVMVolumeGroupStatusSpec,
): DiskHealth {
  const problem = group && getVolumeGroupProblem(group)
  if (problem) return problem

  if (volume.active !== 'active') {
    return { label: 'Inactive', tone: 'warning', glyph: 'warning' }
  }

  return healthy
}

/**
 * Returns the volume groups with a problem whose logical volumes have no
 * device, such as a group missing a physical volume, whose volumes LVM won't
 * activate. Their problem has no device to be shown on.
 */
export function getVolumeGroupsWithoutDevice(
  groups: Resource<LVMVolumeGroupStatusSpec>[],
  logicalVolumes: Resource<LVMLogicalVolumeStatusSpec>[],
  shownLogicalVolumes: string[],
): { id: string; health: DiskHealth }[] {
  return groups.flatMap((group) => {
    const problem = getVolumeGroupProblem(group.spec)
    const volumes = logicalVolumes
      .filter((lv) => lv.spec.vgName === group.spec.name && lv.spec.role === 'public')
      .map((lv) => lv.spec.fullName!)

    if (!problem || volumes.some((v) => shownLogicalVolumes.includes(v))) return []

    const inactive = volumes.length
      ? `\nIts logical volume${volumes.length === 1 ? '' : 's'} ${volumes.join(', ')} ${volumes.length === 1 ? 'is' : 'are'} inactive.`
      : ''

    return [{ id: group.metadata.id!, health: { ...problem, message: problem.message + inactive } }]
  })
}

/** The bits of an NVMe disk's SMART critical warning, from the NVMe spec. */
enum NVMeCriticalWarning {
  SpareBelowThreshold = 1 << 0,
  Temperature = 1 << 1,
  ReliabilityDegraded = 1 << 2,
  ReadOnly = 1 << 3,
  VolatileBackupFailed = 1 << 4,
  PersistentMemoryReadOnly = 1 << 5,
}

const nvmeCriticalWarningMessages: Record<NVMeCriticalWarning, string> = {
  [NVMeCriticalWarning.SpareBelowThreshold]: 'Available spare space is below its threshold.',
  [NVMeCriticalWarning.Temperature]: 'Temperature is outside its thresholds.',
  [NVMeCriticalWarning.ReliabilityDegraded]: 'Reliability is degraded by media or internal errors.',
  [NVMeCriticalWarning.ReadOnly]: 'Media is read-only.',
  [NVMeCriticalWarning.VolatileBackupFailed]: 'The volatile memory backup device has failed.',
  [NVMeCriticalWarning.PersistentMemoryReadOnly]: 'Persistent memory is read-only.',
}

// One line for each bit set in the warning, lowest bit first.
const describeCriticalWarning = (warning: number) =>
  Object.entries(nvmeCriticalWarningMessages)
    .filter(([flag]) => warning & Number(flag))
    .map(([, message]) => message)
    .join('\n') || undefined

/** Returns a disk's health from SMART. */
export function getSMARTHealth(smart: SMARTStatusSpec): DiskHealth {
  if (smart.healthy) return healthy

  // Talos doesn't always say why: an NVMe disk with a critical warning has no message.
  const message =
    smart.message ||
    (smart.critical_warning ? describeCriticalWarning(smart.critical_warning) : undefined)

  return { label: 'Failing', tone: 'danger', glyph: 'danger', message }
}

const formatCount = (n?: number) => (n === undefined ? undefined : n.toLocaleString())
const formatPercent = (n?: number) => (n === undefined ? undefined : `${n}%`)

/** Returns what SMART reports about a disk, beyond its health. */
export function getSMARTDetails(smart: SMARTStatusSpec): DiskDetail[] {
  const details: [string, string | undefined][] = [
    ['Temperature', smart.temperature === undefined ? undefined : `${smart.temperature} °C`],
    ['Power-on hours', formatCount(smart.power_on_hours)],
    ['Power cycles', formatCount(smart.power_cycles)],
    ['Wear', formatPercent(smart.percent_used)],
    ['Available spare', formatPercent(smart.available_spare)],
    ['Media errors', formatCount(smart.media_errors)],
    [
      'Critical warning',
      smart.critical_warning ? `0x${smart.critical_warning.toString(16)}` : undefined,
    ],
    [
      'Failing attributes',
      smart.attributes
        ?.filter((a) => a.failing)
        .map((a) => a.name)
        .join(', ') || undefined,
    ],
  ]

  return details
    .filter((d): d is [string, string] => d[1] !== undefined)
    .map(([label, value]) => ({ label, values: [value] }))
}
