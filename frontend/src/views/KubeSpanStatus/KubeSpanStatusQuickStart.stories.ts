// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { ClusterSpec, ClusterStatusSpec } from '@/api/omni/specs/omni.pb.ts'
import type { ClusterPermissionsSpec } from '@/api/omni/specs/virtual.pb.ts'
import {
  ClusterStatusType,
  ClusterType,
  DefaultNamespace,
  VirtualNamespace,
} from '@/api/resources.ts'

import KubeSpanStatusQuickStart from './KubeSpanStatusQuickStart.vue'

const clusterId = 'my-cluster'

const clusterHandler = createWatchStreamHandler<ClusterSpec>({
  expectedOptions: {
    namespace: DefaultNamespace,
    type: ClusterType,
  },
  initialResources: [
    {
      spec: { talos_version: 'v1.10.0' },
      metadata: { namespace: DefaultNamespace, type: ClusterType, id: clusterId },
    },
  ],
}).handler

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
  return http.post('/omni.resources.ResourceService/Get', () =>
    HttpResponse.json({
      body: JSON.stringify({
        metadata: { namespace: VirtualNamespace, id: clusterId },
        spec,
      }),
    }),
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

export const Default = {
  parameters: {
    msw: {
      handlers: [
        permissionsHandler({ can_manage_config_patches: true }),
        clusterHandler,
        clusterStatusHandler(6),
      ],
    },
  },
} satisfies Story

export const ReadOnly = {
  parameters: {
    msw: {
      handlers: [
        permissionsHandler({ can_manage_config_patches: false }),
        clusterHandler,
        clusterStatusHandler(6),
      ],
    },
  },
} satisfies Story

export const LargeCluster = {
  parameters: {
    msw: {
      handlers: [
        permissionsHandler({ can_manage_config_patches: true }),
        clusterHandler,
        clusterStatusHandler(64),
      ],
    },
  },
} satisfies Story
