// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import pluralize from 'pluralize'
import prettyBytes from 'pretty-bytes'

import type { Resource } from '@/api/grpc'
import type {
  ContainerImageStatusSpec,
  ContainerInstanceStatusSpec,
  ContainerStatusSpec,
} from '@/api/talos/containers.pb'
import { type ContainerMetrics, formatCPU, statsKey } from '@/views/Containers/lib/containerStats'
import { formatSince } from '@/views/Containers/lib/containerStatus'

/** A container, with what Talos reports about its image and its newest instance. */
export interface ContainerRow {
  id: string
  status: ContainerStatusSpec
  image?: ContainerImageStatusSpec
  instance?: ContainerInstanceStatusSpec
  /** When the container entered its state, if Talos says. */
  since?: Date
  /** What its current instance is using, while it runs. */
  metrics?: ContainerMetrics
}

/** Joins the statuses by container: an image status shares the container's ID, an instance names it. */
export function containerRows(
  statuses: Resource<ContainerStatusSpec>[],
  images: Resource<ContainerImageStatusSpec>[],
  instances: Resource<ContainerInstanceStatusSpec>[],
  metrics = new Map<string, ContainerMetrics>(),
): ContainerRow[] {
  const imageById = new Map(images.map((i) => [i.metadata.id, i.spec]))
  const newestInstance = new Map<string, ContainerInstanceStatusSpec>()

  for (const { spec } of instances) {
    if (!spec.containerID) continue

    const current = newestInstance.get(spec.containerID)

    if (!current || (spec.generation ?? 0) > (current.generation ?? 0)) {
      newestInstance.set(spec.containerID, spec)
    }
  }

  return statuses.map(({ metadata, spec: status }) => {
    const id = metadata.id!
    const instance = newestInstance.get(id)

    return {
      id,
      status,
      image: imageById.get(id),
      instance,
      since: sinceTime({ status, instance }),
      metrics: metrics.get(statsKey(id, status.generation)),
    }
  })
}

/** The image reference requested, or the digest Talos resolved when that's all there is. */
export function imageRef(row: ContainerRow) {
  return row.image?.image || row.status.image
}

/**
 * When the container entered its state, from its newest instance: started for a running one,
 * finished for one that has stopped. A container that hasn't started an instance yet has no time.
 */
export function sinceTime({ status, instance }: Pick<ContainerRow, 'status' | 'instance'>) {
  const time = (() => {
    switch (status.state) {
      case 'running':
      case 'stopping':
        return instance?.startedAt
      case 'exited':
      case 'backoff':
      case 'failed':
      case 'completed':
      case 'succeeded':
        return instance?.finishedAt
    }
  })()

  // Go's zero time, which an unset timestamp may come back as
  if (!time || time.startsWith('0001-')) return

  return new Date(time)
}

/**
 * A row's numbers as one phrase, for a stacked row where they have no column headers: "1 restart,
 * 42% CPU, 212 MiB memory, 3h 5m in this state". Readings Talos doesn't have yet are left out.
 */
export function rowSummary({ status, metrics, since }: ContainerRow, now: Date) {
  return [
    pluralize('restart', status.restartCount ?? 0, true),
    metrics?.cpuPercent !== undefined && `${formatCPU(metrics.cpuPercent)} CPU`,
    metrics?.memory !== undefined && `${prettyBytes(metrics.memory, { binary: true })} memory`,
    since && `${formatSince(since, now)} in this state`,
  ]
    .filter(Boolean)
    .join(', ')
}
