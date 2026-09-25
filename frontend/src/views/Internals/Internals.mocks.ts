// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createResourceGetHandler, createWatchStreamHandler } from '@msw/helpers'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterSecretsType,
  ClusterStatusType,
  ClusterType,
  CurrentUserType,
  DefaultNamespace,
  MachineStatusType,
  MetaNamespace,
  ResourceDefinitionType,
  SBCConfigType,
  TalosClusterNamespace,
  TalosMemberType,
  TalosRuntimeNamespace,
  TalosServiceType,
  VirtualNamespace,
} from '@/api/resources'
import {
  resourceDefinitionID,
  type ResourceDefinitionSpec,
} from '@/views/Internals/lib/resourceDefinition'

// Mocks below are generated at import time, before Storybook seeds faker for each story
faker.seed(0)

export const TalosApiCertificateType = 'ApiCertificates.secrets.talos.dev'
export const TalosSecretsNamespace = 'secrets'

function definition(spec: ResourceDefinitionSpec): Resource<ResourceDefinitionSpec> {
  return {
    metadata: {
      namespace: MetaNamespace,
      type: ResourceDefinitionType,
      id: resourceDefinitionID(spec.type!),
      version: '1',
      phase: 'running',
      created: faker.date.recent({ days: 7 }).toISOString(),
      updated: faker.date.recent({ days: 7 }).toISOString(),
    },
    spec,
  }
}

export const omniDefinitions = [
  definition({
    type: ClusterType,
    displayType: 'Cluster',
    defaultNamespace: DefaultNamespace,
    aliases: ['cluster', 'clusters'],
  }),
  definition({
    type: ClusterStatusType,
    displayType: 'ClusterStatus',
    defaultNamespace: DefaultNamespace,
    aliases: ['clusterstatus', 'clusterstatuses'],
  }),
  definition({
    type: ClusterSecretsType,
    displayType: 'ClusterSecrets',
    defaultNamespace: DefaultNamespace,
    aliases: ['clustersecrets'],
    sensitivity: 'sensitive',
  }),
  definition({
    type: MachineStatusType,
    displayType: 'MachineStatus',
    defaultNamespace: DefaultNamespace,
    aliases: ['machinestatus', 'machinestatuses', 'ms'],
  }),
  definition({
    type: CurrentUserType,
    displayType: 'CurrentUser',
    defaultNamespace: VirtualNamespace,
    aliases: ['currentuser', 'currentusers'],
  }),
  definition({
    type: SBCConfigType,
    displayType: 'SBCConfig',
    defaultNamespace: VirtualNamespace,
    aliases: ['sbcconfig', 'sbcconfigs'],
  }),
]

export const talosDefinitions = [
  definition({
    type: TalosServiceType,
    displayType: 'Service',
    defaultNamespace: TalosRuntimeNamespace,
    aliases: ['svc', 'service', 'services'],
  }),
  definition({
    type: TalosMemberType,
    displayType: 'Member',
    defaultNamespace: TalosClusterNamespace,
    aliases: ['member', 'members'],
  }),
  definition({
    type: TalosApiCertificateType,
    displayType: 'ApiCertificate',
    defaultNamespace: TalosSecretsNamespace,
    aliases: ['apicertificate', 'apicertificates'],
    sensitivity: 'sensitive',
  }),
]

/** Serves Talos definitions to requests targeting a node, and Omni's otherwise. */
export const definitionsWatchHandler = createWatchStreamHandler<ResourceDefinitionSpec>({
  expectedOptions: {
    namespace: MetaNamespace,
    type: ResourceDefinitionType,
  },
  initialResources: ({ contextNode }) => (contextNode ? talosDefinitions : omniDefinitions),
}).handler

export const definitionGetHandler = createResourceGetHandler<ResourceDefinitionSpec>({
  expectedOptions: {
    namespace: MetaNamespace,
    type: ResourceDefinitionType,
  },
  resources: [...omniDefinitions, ...talosDefinitions],
})

function machineStatus(spec: MachineStatusSpec): Resource<MachineStatusSpec> {
  return {
    metadata: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id: faker.string.uuid(),
    },
    spec,
  }
}

export const machineStatuses = [
  machineStatus({
    network: { hostname: 'talos-default-controlplane-1' },
    cluster: 'talos-default',
    connected: true,
  }),
  machineStatus({
    network: { hostname: 'talos-default-worker-1' },
    cluster: 'talos-default',
    connected: true,
  }),
  machineStatus({
    network: { hostname: 'talos-unallocated-1' },
    maintenance: true,
    connected: true,
  }),
  machineStatus({
    network: { hostname: 'talos-default-worker-2' },
    cluster: 'talos-default',
    connected: false,
  }),
]

export const machineStatusesHandler = createWatchStreamHandler<MachineStatusSpec>({
  expectedOptions: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  initialResources: machineStatuses,
}).handler

/** Everything the breadcrumbs shared by all resource browser views need. */
export const commonHandlers = [
  definitionsWatchHandler,
  definitionGetHandler,
  machineStatusesHandler,
]
