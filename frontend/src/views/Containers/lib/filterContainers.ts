// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type ContainerRow, imageRef } from '@/views/Containers/lib/containerRows'

export const containerSortOptions = [
  { label: 'Name ⬆', value: 'name' },
  { label: 'Name ⬇', value: '-name' },
  { label: 'Restarts ⬇', value: '-restarts' },
  { label: 'Restarts ⬆', value: 'restarts' },
] as const

export type ContainerSort = (typeof containerSortOptions)[number]['value']

export const containerHealthFilterOptions = [
  { label: 'All' },
  { label: 'Healthy', value: 'healthy' },
  { label: 'Pulling', value: 'pulling' },
  { label: 'Pending', value: 'pending' },
  { label: 'Degraded', value: 'degraded' },
]

const byName = (a: ContainerRow, b: ContainerRow) => a.id.localeCompare(b.id)

const compare: Record<string, (a: ContainerRow, b: ContainerRow) => number> = {
  name: byName,
  restarts: (a, b) => (a.status.restartCount ?? 0) - (b.status.restartCount ?? 0) || byName(a, b),
}

/**
 * Filters and sorts the containers. The search matches a substring of the name, image, state or
 * error, ignoring case.
 */
export function filterContainers(
  rows: ContainerRow[],
  {
    search = '',
    health,
    sort = 'name',
  }: { search?: string; health?: string; sort?: ContainerSort },
) {
  const query = search.trim().toLowerCase()
  const descending = sort.startsWith('-')
  const cmp = compare[descending ? sort.slice(1) : sort]

  return rows
    .filter((r) => !health || r.status.health === health)
    .filter(
      (r) =>
        !query ||
        [r.id, imageRef(r), r.status.state, r.status.error].some((field) =>
          field?.toLowerCase().includes(query),
        ),
    )
    .sort((a, b) => (descending ? cmp(b, a) : cmp(a, b)))
}
