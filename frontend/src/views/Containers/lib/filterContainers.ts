// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { ResourceSortOption } from '@/methods/resource/useResourceSort'
import { type ContainerRow, imageRef } from '@/views/Containers/lib/containerRows'

export const containerSortOptions: ResourceSortOption[] = [
  { id: 'name', desc: 'Name ⬆' },
  { id: 'name', desc: 'Name ⬇', descending: true },
  { id: 'restarts', desc: 'Restarts ⬇', descending: true },
  { id: 'restarts', desc: 'Restarts ⬆' },
  { id: 'cpu', desc: 'CPU ⬇', descending: true },
  { id: 'memory', desc: 'Memory ⬇', descending: true },
]

export const containerHealthFilterOptions = [
  { label: 'All' },
  { label: 'Healthy', value: 'healthy' },
  { label: 'Pulling', value: 'pulling' },
  { label: 'Pending', value: 'pending' },
  { label: 'Degraded', value: 'degraded' },
]

const byName = (a: ContainerRow, b: ContainerRow) => a.id.localeCompare(b.id)

const byMetric = (a = -1, b = -1) => a - b

const compare: Record<string, (a: ContainerRow, b: ContainerRow) => number> = {
  name: byName,
  restarts: (a, b) => (a.status.restartCount ?? 0) - (b.status.restartCount ?? 0) || byName(a, b),
  // A container with no reading sorts as the lowest, so last when sorting by most used
  cpu: (a, b) => byMetric(a.metrics?.cpuPercent, b.metrics?.cpuPercent) || byName(b, a),
  memory: (a, b) => byMetric(a.metrics?.memory, b.metrics?.memory) || byName(b, a),
}

/**
 * Filters and sorts the containers. The search matches a substring of the name, image, state or
 * error, ignoring case. A sort field it doesn't know, such as one edited into the URL, sorts by
 * name.
 */
export function filterContainers(
  rows: ContainerRow[],
  {
    search = '',
    health,
    sortByField = 'name',
    sortDescending = false,
  }: { search?: string; health?: string; sortByField?: string; sortDescending?: boolean },
) {
  const query = search.trim().toLowerCase()
  const cmp = compare[sortByField] ?? byName

  return rows
    .filter((r) => !health || r.status.health === health)
    .filter(
      (r) =>
        !query ||
        [r.id, imageRef(r), r.status.state, r.status.error].some((field) =>
          field?.toLowerCase().includes(query),
        ),
    )
    .sort((a, b) => (sortDescending ? cmp(b, a) : cmp(a, b)))
}
