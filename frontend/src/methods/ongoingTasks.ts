// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, effectScope } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import {
  KubernetesUpgradeStatusSpecPhase,
  type OngoingTaskSpec,
  SecretRotationSpecComponent,
} from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

type OngoingTaskKind = 'kubernetes' | 'talos' | 'machine' | 'secrets' | 'destroy' | 'unknown'

/**
 * Structured, presentation-agnostic description of an ongoing task. Both the
 * header popover (which renders version "pills") and the home page panel (which
 * renders an inline summary string) derive from this so the two stay in sync.
 */
interface OngoingTaskDescription {
  kind: OngoingTaskKind
  /** Short action label, e.g. "Upgrade Kubernetes", "Rotating secrets". */
  action: string
  /** Rotated component, when applicable (e.g. "Talos CA"). */
  component?: string
  /** Version being upgraded from (only when a target version is known). */
  fromVersion?: string
  /** Version being upgraded to. */
  toVersion?: string
  /** Target version when an upgrade is being reverted. */
  revertingTo?: string
}

const taskScope = effectScope(true)

const tasks = taskScope.run(() => {
  const { data } = useResourceWatch<OngoingTaskSpec>(() => ({
    resource: {
      namespace: EphemeralNamespace,
      type: OngoingTaskType,
    },
    runtime: Runtime.Omni,
  }))

  return data
})!

export function useOngoingTasks() {
  return {
    data: computed(() =>
      tasks.value.map((item) => ({
        item,
        desc: describeOngoingTask(item),
        summary: ongoingTaskSummary(item),
      })),
    ),
  }
}

function secretComponentName(component?: SecretRotationSpecComponent) {
  switch (component) {
    case SecretRotationSpecComponent.TALOS_CA:
      return 'Talos CA'
    case SecretRotationSpecComponent.KUBERNETES_CA:
      return 'Kubernetes CA'
    default:
      return undefined
  }
}

const KIND_NAME = {
  kubernetes: 'Kubernetes',
  talos: 'Talos',
  machine: 'Machine',
}

function getPreviousVersion({ spec }: Resource<OngoingTaskSpec>) {
  return (
    spec.kubernetes_upgrade?.last_upgrade_version ??
    spec.talos_upgrade?.last_upgrade_version ??
    spec.machine_upgrade?.current_schematic_id
  )
}

function getCurrentVersion({ spec }: Resource<OngoingTaskSpec>) {
  return (
    spec.kubernetes_upgrade?.current_upgrade_version ??
    spec.talos_upgrade?.current_upgrade_version ??
    spec.machine_upgrade?.current_schematic_id
  )
}

/** Maps a task's oneof payload to a structured, renderable description. */
function describeOngoingTask(item: Resource<OngoingTaskSpec>): OngoingTaskDescription {
  const { spec } = item

  if (spec.destroy) {
    return {
      kind: 'destroy',
      action: 'Destroying',
    }
  }

  if (spec.secrets_rotation) {
    return {
      kind: 'secrets',
      action: 'Rotating secrets',
      component: secretComponentName(spec.secrets_rotation.component),
    }
  }

  const upgrade = spec.kubernetes_upgrade ?? spec.talos_upgrade ?? spec.machine_upgrade
  if (upgrade) {
    const kind: OngoingTaskKind = spec.kubernetes_upgrade
      ? 'kubernetes'
      : spec.talos_upgrade
        ? 'talos'
        : 'machine'
    const action = `Upgrade ${KIND_NAME[kind]}`

    const toVersion = getCurrentVersion(item)
    if (toVersion) {
      return {
        kind,
        action,
        fromVersion: getPreviousVersion(item),
        toVersion,
      }
    }

    const upgradePhase = spec.kubernetes_upgrade?.phase ?? spec.talos_upgrade?.phase
    if (upgradePhase === KubernetesUpgradeStatusSpecPhase.Reverting) {
      return {
        kind,
        action,
        revertingTo: (spec.kubernetes_upgrade ?? spec.talos_upgrade)?.last_upgrade_version,
      }
    }

    return {
      kind: 'machine',
      action: 'Updating machine schematics',
    }
  }

  return {
    kind: 'unknown',
    action: 'In progress',
  }
}

/** Single-line summary of a task, for compact/inline presentations. */
function ongoingTaskSummary(item: Resource<OngoingTaskSpec>) {
  const desc = describeOngoingTask(item)

  if (desc.kind !== 'destroy' && desc.kind !== 'secrets' && desc.kind !== 'unknown') {
    const name = KIND_NAME[desc.kind]

    if (desc.fromVersion && desc.toVersion) {
      return `${name} ${desc.fromVersion} → ${desc.toVersion}`
    }

    if (desc.revertingTo) {
      return `${name} reverting to ${desc.revertingTo}`
    }
  }

  if (desc.component) {
    return `${desc.action} · ${desc.component}`
  }

  return desc.action
}
