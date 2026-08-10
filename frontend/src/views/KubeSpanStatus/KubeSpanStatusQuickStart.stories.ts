// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type {
  ClusterConfigVersionSpec,
  ClusterSpec,
  ClusterStatusSpec,
} from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec, VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterConfigVersionType,
  ClusterPermissionsType,
  ClusterStatusType,
  ClusterType,
  DefaultNamespace,
  VersionContractType,
  VirtualNamespace,
} from '@/api/resources'

import KubeSpanStatusQuickStart from './KubeSpanStatusQuickStart.vue'

const clusterId = 'my-cluster'
const talosVersion = 'v1.10.0'

const clusterHandler = createWatchStreamHandler<ClusterSpec>({
  expectedOptions: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: clusterId,
  },
  initialResources: [
    {
      spec: {
        talos_version: talosVersion,
      },
      metadata: {
        namespace: DefaultNamespace,
        type: ClusterType,
        id: clusterId,
      },
    },
  ],
}).handler

const clusterConfigHandler = createWatchStreamHandler<ClusterConfigVersionSpec>({
  expectedOptions: {
    namespace: DefaultNamespace,
    type: ClusterConfigVersionType,
    id: clusterId,
  },
  initialResources: [
    {
      spec: { version: talosVersion },
      metadata: {
        namespace: DefaultNamespace,
        type: ClusterConfigVersionType,
        id: clusterId,
      },
    },
  ],
}).handler

function versionContractHandler(spec: VersionContractSpec) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { id, type, namespace } = await request.clone().json()

      if (id !== talosVersion || type !== VersionContractType || namespace !== VirtualNamespace)
        return

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: {
            namespace: VirtualNamespace,
            type: VersionContractType,
            id: talosVersion,
          },
          spec,
        } as Resource<VersionContractSpec>),
      })
    },
  )
}

function clusterStatusHandler(total: number) {
  return createWatchStreamHandler<ClusterStatusSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: ClusterStatusType,
    },
    initialResources: [
      {
        spec: { machines: { total } },
        metadata: { namespace: DefaultNamespace, type: ClusterStatusType, id: clusterId },
      },
    ],
  }).handler
}

function permissionsHandler(spec: ClusterPermissionsSpec) {
  return http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { id, type, namespace } = await request.clone().json()

      if (id !== clusterId || type !== ClusterPermissionsType || namespace !== VirtualNamespace)
        return

      return HttpResponse.json({
        body: JSON.stringify({
          metadata: {
            namespace: VirtualNamespace,
            type: ClusterPermissionsType,
            id: clusterId,
          },
          spec,
        } as Resource<ClusterPermissionsSpec>),
      })
    },
  )
}

const meta: Meta<typeof KubeSpanStatusQuickStart> = {
  component: KubeSpanStatusQuickStart,
  parameters: {
    layout: 'fullscreen',
  },
  args: {
    clusterId,
  },
  decorators: [
    () => ({
      components: { KubeSpanStatusQuickStart },
      template: '<div class="h-screen"><story /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof KubeSpanStatusQuickStart>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(
      versionContractHandler({ kube_span_multidoc_config: true }),
      permissionsHandler({ can_manage_config_patches: true }),
      clusterHandler,
      clusterConfigHandler,
      clusterStatusHandler(6),
    )
  },
}

export const NoMultidoc: Story = {
  beforeEach({ msw }) {
    msw.use(
      versionContractHandler({ kube_span_multidoc_config: false }),
      permissionsHandler({ can_manage_config_patches: true }),
      clusterHandler,
      clusterConfigHandler,
      clusterStatusHandler(6),
    )
  },
}

export const ReadOnly: Story = {
  beforeEach({ msw }) {
    msw.use(
      versionContractHandler({ kube_span_multidoc_config: true }),
      permissionsHandler({ can_manage_config_patches: false }),
      clusterHandler,
      clusterConfigHandler,
      clusterStatusHandler(6),
    )
  },
}

export const LargeCluster: Story = {
  beforeEach({ msw }) {
    msw.use(
      versionContractHandler({ kube_span_multidoc_config: true }),
      permissionsHandler({ can_manage_config_patches: true }),
      clusterHandler,
      clusterConfigHandler,
      clusterStatusHandler(64),
    )
  },
}
