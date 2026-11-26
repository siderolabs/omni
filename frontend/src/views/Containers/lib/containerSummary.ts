// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Resource } from '@/api/grpc'
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

/** Talos's health rollups, worst first. A health newer than this list sorts after them. */
const healthOrder = ['degraded', 'pending', 'pulling', 'healthy']

/** How many containers have each health, worst first, leaving out the ones nobody has. */
export function healthCounts(statuses: Resource<ContainerStatusSpec>[]) {
  const counts = new Map<string, number>()

  for (const { spec } of statuses) {
    const health = spec.health || ''

    counts.set(health, (counts.get(health) ?? 0) + 1)
  }

  const rank = (health: string) => {
    const i = healthOrder.indexOf(health)

    return i === -1 ? healthOrder.length : i
  }

  return [...counts]
    .map(([health, count]) => ({ health, count }))
    .sort((a, b) => rank(a.health) - rank(b.health) || a.health.localeCompare(b.health))
}

/** The containers held back by an unmet gate, with the first gate and how many more there are. */
export function blockedContainers(statuses: Resource<ContainerStatusSpec>[]) {
  return statuses
    .flatMap(({ metadata, spec: { waitingFor = [] } }) =>
      waitingFor.length
        ? [{ id: metadata.id!, gate: waitingFor[0], more: waitingFor.length - 1 }]
        : [],
    )
    .sort((a, b) => a.id.localeCompare(b.id))
}
