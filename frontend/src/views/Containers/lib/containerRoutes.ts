// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { LocationQueryRaw } from 'vue-router'

import { TalosContainersLogPrefix } from '@/api/resources'

/**
 * A machine's container list. Its search, filter and sort live in the query, so a link back to it
 * passes the query on to land where the list was left.
 */
export const containersRoute = (cluster: string, machine: string, query?: LocationQueryRaw) => ({
  name: 'NodeContainers' as const,
  params: { cluster, machine },
  query,
})

/** One container. The list's query rides along, for the way back to the list. */
export const containerRoute = (
  cluster: string,
  machine: string,
  container: string,
  query?: LocationQueryRaw,
) => ({
  name: 'NodeContainer' as const,
  params: { cluster, machine, container },
  query,
})

/** A container's logs, which the Logs tab lists among the services under a prefix. */
export const containerLogsRoute = (cluster: string, machine: string, container: string) => ({
  name: 'NodeLogs' as const,
  params: { cluster, machine, service: `${TalosContainersLogPrefix}${container}` },
})
