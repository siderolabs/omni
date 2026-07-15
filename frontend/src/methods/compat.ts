// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { parse, SemVer } from 'semver'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { LifecycleServiceMinTalosVersion, MachineStatusLabelInstalled } from '@/api/resources'

const [minTalosMajor, minTalosMinor] = LifecycleServiceMinTalosVersion.split('.').map(Number)

export type CompatResult = {
  ok: boolean
  willAutoInstall: boolean
  reason: string | null
}

const okResult: CompatResult = { ok: true, willAutoInstall: false, reason: null }
const okWithInstall = (version: string): CompatResult => ({
  ok: true,
  willAutoInstall: true,
  reason: `Omni will install Talos v${version} to this machine when it joins the cluster.`,
})

function supportsMaintenanceInstall(v: SemVer): boolean {
  if (v.major > minTalosMajor) return true
  if (v.major < minTalosMajor) return false
  return v.minor >= minTalosMinor
}

export function machineCompatibleWithCluster(
  machine: Resource<MachineStatusSpec>,
  clusterVersionStr: string,
): CompatResult {
  const clusterVersion = parse(clusterVersionStr)
  const machineVersion = parse(machine.spec.talos_version)

  if (!machineVersion) {
    return {
      ok: false,
      willAutoInstall: false,
      reason: "The machine's Talos version could not be determined.",
    }
  }

  if (!clusterVersion) {
    return {
      ok: false,
      willAutoInstall: false,
      reason: 'The cluster Talos version could not be determined.',
    }
  }

  const inAgentMode = !!machine.spec.schematic?.in_agent_mode
  if (inAgentMode) {
    return okResult
  }

  // Refuse downgrade.
  if (
    machineVersion.major > clusterVersion.major ||
    (machineVersion.major === clusterVersion.major && machineVersion.minor > clusterVersion.minor)
  ) {
    return {
      ok: false,
      willAutoInstall: false,
      reason:
        'The machine has a newer Talos version installed: downgrade is not allowed. Upgrade the machine or change the cluster Talos version.',
    }
  }

  const installed = machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined
  if (installed) {
    return okResult
  }

  // Not-installed, both sides >= 1.13: Omni installs Talos explicitly via LifecycleService, regardless of minor match.
  if (supportsMaintenanceInstall(machineVersion) && supportsMaintenanceInstall(clusterVersion)) {
    return okWithInstall(clusterVersionStr.replace(/^v/, ''))
  }

  // Below 1.13 there is no explicit install mechanism: config-apply can only install within the same minor.
  if (
    machineVersion.major === clusterVersion.major &&
    machineVersion.minor === clusterVersion.minor
  ) {
    return okResult
  }

  return {
    ok: false,
    willAutoInstall: false,
    reason:
      'The machine running from ISO or PXE must have the same major and minor version as the cluster it is going to be added to. Please use another ISO or change the cluster Talos version.',
  }
}
