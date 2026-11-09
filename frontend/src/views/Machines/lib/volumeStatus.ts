// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Resource } from '@/api/grpc'
import type { VolumeStatusSpec } from '@/api/talos/block.pb'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'

// Phases a volume reaches once it is prepared, which is when encryption is set up.
const preparedPhases = new Set(['prepared', 'ready', 'closed'])

/** Whether the volume got as far as being prepared, so its encryption is known. */
export function isVolumePrepared(phase?: string): boolean {
  return preparedPhases.has(phase ?? '')
}

type VolumeStatus = Resource<VolumeStatusSpec>

export interface VolumePhaseAppearance extends StatusAppearance {
  label: string
}

const phaseAppearances: Record<string, VolumePhaseAppearance> = {
  ready: { label: 'Ready', tone: 'success', glyph: 'success' },
  failed: { label: 'Failed', tone: 'danger', glyph: 'danger' },
  // Discovery isn't complete yet, or an earlier provisioning wave isn't done.
  waiting: { label: 'Waiting', tone: 'info', glyph: 'progress' },
  located: { label: 'Located', tone: 'info', glyph: 'progress' },
  provisioned: { label: 'Provisioned', tone: 'info', glyph: 'progress' },
  prepared: { label: 'Prepared', tone: 'info', glyph: 'progress' },
  // Not found, and not configured to be created: expected before a machine is
  // installed, so not drawn as a problem.
  missing: { label: 'Missing', tone: 'info', glyph: 'neutral' },
  closed: { label: 'Closed', tone: 'info', glyph: 'neutral' },
}

export function getVolumePhaseAppearance(phase?: string): VolumePhaseAppearance {
  return (
    (phase && phaseAppearances[phase]) || {
      label: phase || 'Unknown',
      tone: 'info',
      glyph: 'unknown',
    }
  )
}

/** Returns the failed volumes, by ID. A failed volume may never have been located on a disk. */
export function getFailedVolumes(statuses: VolumeStatus[]): VolumeStatus[] {
  return statuses
    .filter((s) => s.spec.phase === 'failed')
    .sort((a, b) => a.metadata.id!.localeCompare(b.metadata.id!))
}
