// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import {
  InfraProviderLabelPrefix,
  LabelCluster,
  LabelControlPlaneRole,
  LabelInfraProviderID,
  LabelWorkerRole,
  MachineStatusLabelArch,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelCores,
  MachineStatusLabelCPU,
  MachineStatusLabelDisconnected,
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
import type { IconType } from '@/components/common/Icon/TIcon.vue'

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
  labelClass: string
  removable?: boolean
  description?: string
  icon?: IconType
}

const labelClasses: Record<string, string> = {
  [LabelCluster]: 'label-light1',
  [MachineStatusLabelAvailable]: 'label-yellow',
  [MachineStatusLabelInvalidState]: 'label-red',
  [MachineStatusLabelConnected]: 'label-green',
  [MachineStatusLabelDisconnected]: 'label-red',
  [MachineStatusLabelPlatform]: 'label-blue1',
  [MachineStatusLabelCores]: 'label-cyan',
  [MachineStatusLabelMem]: 'label-blue2',
  [MachineStatusLabelStorage]: 'label-violet',
  [MachineStatusLabelNet]: 'label-blue3',
  [MachineStatusLabelCPU]: 'label-orange',
  [MachineStatusLabelArch]: 'label-light2',
  [MachineStatusLabelRegion]: 'label-light3',
  [MachineStatusLabelZone]: 'label-light4',
  [MachineStatusLabelInstance]: 'label-light5',
  [MachineStatusLabelTalosVersion]: 'label-light7',
  [LabelControlPlaneRole]: 'label-yellow',
  [LabelWorkerRole]: 'label-yellow',
}

export const getLabelClass = (labelKey: string) => {
  return labelClasses[labelKey] ?? 'label-light6'
}

export const addLabel = (dest: Label[], label: Label) => {
  if (dest.find((l) => l.value === label.value && l.key === label.key)) {
    return
  }

  dest.push({
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

export const getLabelFromID = (key: string, value: string): Label => {
  const label: Label = {
    key,
    id: key.replace(new RegExp(`^${SystemLabelPrefix}`), ''),
    value,
    labelClass: getLabelClass(key),
    removable: !key.startsWith(SystemLabelPrefix),
    description: labelDescriptions[key],
  }

  if (key.startsWith(InfraProviderLabelPrefix) && key !== LabelInfraProviderID) {
    const parts = label.id.split('/')

    return {
      ...label,
      id: parts.at(-1) ?? '',
      labelClass: 'label-green',
      description: `Defined by the infra provider "${parts[1] ?? ''}""`,
      icon: 'server-network',
    }
  }

  return label
}
