// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import prettyBytes from 'pretty-bytes'

import type {
  ContainerDependsOnSpec,
  ContainerHealthCheckSpec,
  ContainerInstanceStatusSpec,
  ContainerMountSpec,
  ContainerResourcesSpec,
  ContainerRestartSpec,
  ContainerRunAsSpec,
} from '@/api/talos/containers.pb'
import { formatFullDateTime } from '@/methods/time'

const bytes = (value: number) => prettyBytes(value, { binary: true })

/** The user a container runs as: "1000:1000", or the image's own user when the spec sets none. */
export function runAsText(runAs?: ContainerRunAsSpec) {
  if (runAs?.uid === undefined && runAs?.gid === undefined) return 'Image default'

  return `${runAs.uid ?? 'image default'}:${runAs.gid ?? 'image default'}`
}

/** Limits as one line; zero or unset means unlimited, except swap, where zero means none. */
export function limitsText(resources?: ContainerResourcesSpec) {
  const limits = [
    resources?.cpuLimit && `CPU ${resources.cpuLimit / 1000} cores`,
    resources?.memoryLimit && `memory ${bytes(resources.memoryLimit)}`,
    resources?.swapLimit !== undefined &&
      (resources.swapLimit ? `swap ${bytes(resources.swapLimit)}` : 'no swap'),
  ].filter(Boolean)

  return limits.length ? limits.join(', ') : 'None'
}

/** The restart policy, with its retry limit and backoff when it restarts at all. */
export function restartText(restart?: ContainerRestartSpec) {
  const policy = restart?.policy || 'always'

  if (policy === 'never') return 'Never'

  const name = policy === 'on-failure' ? 'On failure' : 'Always'
  const retries =
    restart?.maxRetries === undefined ? 'no retry limit' : `at most ${restart.maxRetries} retries`
  const backoff = restart?.delay && `backoff ${restart.delay} to ${restart.maxDelay || '?'}`

  return [name, retries, backoff].filter(Boolean).join(', ')
}

/** What a health check probes, on the container's loopback: "HTTP :8080/healthz". */
export function healthCheckTarget(check: ContainerHealthCheckSpec) {
  switch (check.kind) {
    case 'http':
      return `HTTP :${check.port}${check.path || '/'}`
    case 'tcp':
      return `TCP :${check.port}`
    case 'grpc':
      return `gRPC :${check.port}${check.grpcService ? ` ${check.grpcService}` : ''}`
    default:
      return check.kind ? `${check.kind} :${check.port}` : 'None'
  }
}

/** Where a mount comes from: the volume, host path, or a tmpfs and its size. */
export function mountSource(mount: ContainerMountSpec) {
  switch (mount.kind) {
    case 'volume':
      return `volume ${mount.volumeName}`
    case 'hostPath':
      return mount.source ?? ''
    case 'tmpfs':
      return mount.size ? `tmpfs, ${bytes(mount.size)}` : 'tmpfs'
    default:
      return [mount.kind, mount.source].filter(Boolean).join(' ')
  }
}

/** Every gate a container declared in `dependsOn`, one line each. */
export function dependsOnList(dependsOn?: ContainerDependsOnSpec) {
  return [
    ...(dependsOn?.networks ?? []).map((network) => `network ${network}`),
    ...(dependsOn?.time ? ['time'] : []),
    ...(dependsOn?.paths ?? []).map((path) => `path: ${path}`),
    ...(dependsOn?.containers ?? []).map(
      ({ name, condition }) => `container ${name} ${condition || 'running'}`,
    ),
  ]
}

/** A run's start or finish, or nothing when it hasn't; an unset time can come back as Go's zero time. */
export function runTime(time?: string) {
  return time && !time.startsWith('0001-') ? formatFullDateTime(time) : ''
}

/**
 * A run's numbers as one phrase, for a stacked row where they have no column headers: "PID 4021,
 * exit code 1, started …, finished …". What the run doesn't have is left out.
 */
export function runSummary({ pid, exitCode, startedAt, finishedAt }: ContainerInstanceStatusSpec) {
  const started = runTime(startedAt)
  const finished = runTime(finishedAt)

  return [
    pid && `PID ${pid}`,
    exitCode !== undefined && `exit code ${exitCode}`,
    started && `started ${started}`,
    finished && `finished ${finished}`,
  ]
    .filter(Boolean)
    .join(', ')
}
