// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { useRouteQuery } from '@vueuse/router'

import {
  InfraProviderLabelPrefix,
  LabelCluster,
  LabelControlPlaneRole,
  LabelEnterprise,
  LabelInfraProviderID,
  LabelMachineSet,
  LabelWorkerRole,
  MachineStatusLabelArch,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelCores,
  MachineStatusLabelCPU,
  MachineStatusLabelDisconnected,
  MachineStatusLabelFIPS,
  MachineStatusLabelInstance,
  MachineStatusLabelInvalidState,
  MachineStatusLabelMem,
  MachineStatusLabelNet,
  MachineStatusLabelPlatform,
  MachineStatusLabelRegion,
  MachineStatusLabelStorage,
  MachineStatusLabelTalosVersion,
  MachineStatusLabelZone,
  SystemLabelPrefix,
} from '@/api/resources'
import type { IconType } from '@/components/Icon/TIcon.vue'

export const parseLabels = (...labels: string[]): Record<string, string> => {
  const labelsMap: Record<string, string> = {}

  for (const label of labels) {
    const parts = label.split(':', 2)

    labelsMap[parts[0].trim()] = (parts[1] ?? '').trim()
  }

  return labelsMap
}

export type Label = {
  key: string
  id: string
  value: string
  removable?: boolean
  system?: boolean
  tone?: 'danger'
  description?: string
  icon?: IconType
}

const labelIcons: Record<string, IconType> = {
  // Cluster membership
  [LabelCluster]: 'clusters',
  [LabelMachineSet]: 'clusters',
  [LabelControlPlaneRole]: 'clusters',
  [LabelWorkerRole]: 'clusters',
  [MachineStatusLabelAvailable]: 'box',

  // Talos
  [MachineStatusLabelTalosVersion]: 'talos',
  [LabelEnterprise]: 'talos',
  [MachineStatusLabelFIPS]: 'talos',

  // Connection state
  [MachineStatusLabelConnected]: 'cloud-connection',
  [MachineStatusLabelDisconnected]: 'no-connection',

  // Hardware
  [MachineStatusLabelPlatform]: 'cpu-chip',
  [MachineStatusLabelCores]: 'cpu-chip',
  [MachineStatusLabelMem]: 'cpu-chip',
  [MachineStatusLabelStorage]: 'cpu-chip',
  [MachineStatusLabelNet]: 'cpu-chip',
  [MachineStatusLabelCPU]: 'cpu-chip',
  [MachineStatusLabelArch]: 'cpu-chip',
  [MachineStatusLabelRegion]: 'cpu-chip',
  [MachineStatusLabelZone]: 'cpu-chip',
  [MachineStatusLabelInstance]: 'cpu-chip',

  // Infra provider
  [LabelInfraProviderID]: 'server-network',

  // Other
  [MachineStatusLabelInvalidState]: 'exclamation-triangle',
}

export function useLabelRouteQuery() {
  return useRouteQuery<string, Label[]>('labels', '', {
    transform: {
      get(val) {
        if (!val) return []

        try {
          const parsed = JSON.parse(window.atob(val))

          return Array.isArray(parsed) ? parsed : []
        } catch {
          return []
        }
      },
      set(val) {
        if (!val.length) return ''

        return window.btoa(JSON.stringify(val))
      },
    },
  })
}

export const addLabel = (dest: Label[], label: Label) => {
  if (dest.find((l) => l.value === label.value && l.key === label.key)) {
    return dest
  }

  return dest.concat({
    ...label,
    id: !label.value ? `has label: ${label.id}` : label.id,
  })
}

export const selectors = (labels: Label[]) => {
  if (labels.length === 0) {
    return
  }

  return labels.map((label: Label) => {
    if (label.value === '') {
      return label.key
    }

    const value = sanitizeLabelValue(label.value)

    return `${label.key}=${value}`
  })
}

export const sanitizeLabelValue = (value: string): string => {
  if (value.includes(',')) {
    // Escape any double quotes in the value.
    value = value.replace(/"/g, '\\"')

    // Wrap the value in quotes.
    return `"${value}"`
  }

  return value
}

const labelDescriptions: Record<string, string> = {
  [MachineStatusLabelInvalidState]:
    'The machine is expected to be unallocated, but still has the configuration of a cluster.\nIt might be required to wipe the machine bypassing Omni.',
}

const dangerLabels = new Set([MachineStatusLabelInvalidState, MachineStatusLabelDisconnected])

export const getLabelFromID = (key: string, value: string): Label => {
  const label: Label = {
    key,
    id: key.replace(new RegExp(`^${SystemLabelPrefix}`), ''),
    value,
    removable: !key.startsWith(SystemLabelPrefix),
    system: key.startsWith(SystemLabelPrefix),
    tone: dangerLabels.has(key) ? 'danger' : undefined,
    description: labelDescriptions[key],
    icon: labelIcons[key] ?? (key.startsWith(SystemLabelPrefix) ? undefined : 'tag'),
  }

  if (key.startsWith(InfraProviderLabelPrefix) && key !== LabelInfraProviderID) {
    const parts = label.id.split('/')

    return {
      ...label,
      id: parts.at(-1) ?? '',
      description: `Defined by the infra provider "${parts[1] ?? ''}"`,
      icon: 'server-network',
    }
  }

  return label
}
