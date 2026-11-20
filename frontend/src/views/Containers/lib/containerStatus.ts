// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'

/** The name shown for a container state. */
export function stateName(state?: string) {
  switch (state) {
    case 'pending':
      return 'Pending'
    case 'pulling':
      return 'Pulling'
    case 'starting':
      return 'Starting'
    case 'running':
      return 'Running'
    case 'exited':
      return 'Exited'
    case 'backoff':
      return 'Backoff'
    case 'stopping':
      return 'Stopping'
    case 'completed':
      return 'Completed'
    // completed's new name in Talos #14823
    case 'succeeded':
      return 'Succeeded'
    case 'failed':
      return 'Failed'
    case undefined:
    case '':
      return 'Unknown'
    // a state newer than this list reads as Talos says it
    default:
      return state
  }
}

/**
 * The pill for a container state. States changing as expected are in progress, which is info.
 * A container still running or settling while Talos reports it degraded needs attention.
 */
export function stateAppearance(state?: string, health?: string): StatusAppearance {
  switch (state) {
    case 'pending':
    case 'pulling':
    case 'starting':
    case 'stopping':
      return health === 'degraded'
        ? { tone: 'warning', glyph: 'warning' }
        : { tone: 'info', glyph: 'progress' }
    case 'running':
    case 'completed':
    case 'succeeded':
      return health === 'degraded'
        ? { tone: 'warning', glyph: 'warning' }
        : { tone: 'success', glyph: 'success' }
    case 'exited':
    case 'backoff':
    case 'failed':
      return { tone: 'danger', glyph: 'danger' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}

/** The first unmet gate of a waiting container, counting the rest: "path: /run/x +2". */
export function waitingText(waitingFor?: string[]) {
  if (!waitingFor?.length) return

  const extra = waitingFor.length - 1

  return extra ? `${waitingFor[0]} +${extra}` : waitingFor[0]
}

/** Splits an image reference so the registry path can be cut first: the name and tag stay visible. */
export function splitImage(image: string) {
  const slash = image.lastIndexOf('/', image.includes('@') ? image.indexOf('@') : undefined)

  return { path: image.slice(0, slash + 1), name: image.slice(slash + 1) }
}

/** Time in a state, in at most two units: "12d 6h", "3h 5m", "3m", "41s". */
export function formatSince(from: Date, now: Date) {
  const seconds = Math.max(0, Math.floor((now.getTime() - from.getTime()) / 1000))
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (days) return hours ? `${days}d ${hours}h` : `${days}d`
  if (hours) return minutes ? `${hours}h ${minutes}m` : `${hours}h`
  if (minutes) return `${minutes}m`

  return `${seconds}s`
}
