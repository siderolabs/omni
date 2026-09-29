// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'
import { defineComponent, h } from 'vue'
import { RouterView, useRoute } from 'vue-router'

import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineIdentityType,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelControlPlaneRole,
  LabelWorkerRole,
  TalosKubeSpanLinkName,
  TalosKubeSpanNamespace,
  TalosKubeSpanPeerStatusType,
  TalosLinkStatusType,
  TalosNetworkNamespace,
} from '@/api/resources'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'
import type { LinkStatusSpec } from '@/api/talos/network.pb'

import KubeSpanStatus from './KubeSpanStatus.vue'

faker.seed(0)

const clusterId = faker.string.uuid()

const machines = faker.helpers.multiple(
  () => ({
    id: faker.string.uuid(),
    nodename: `talos-${faker.string.alphanumeric({ length: 6, casing: 'lower' })}`,
  }),
  { count: 8 },
)

const KubeSpanStatusWithRoute = defineComponent(() => {
  const route = useRoute<'NodeKubeSpanStatus'>()

  return () =>
    h(KubeSpanStatus, {
      clusterId,
      machineId: route.params.machine ?? machines[0].id,
    })
})

const meta: Meta<typeof KubeSpanStatus> = {
  component: KubeSpanStatus,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    vueRouter([{ path: '/:cluster/:machine', component: RouterView }], {
      initialRoute: `/${clusterId}/${machines[0].id}`,
    }),
    () => ({
      components: { KubeSpanStatusWithRoute },
      template: '<div class="h-screen"><KubeSpanStatusWithRoute /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof KubeSpanStatus>

const handlers = [
  createWatchStreamHandler<LinkStatusSpec>({
    expectedOptions: {
      namespace: TalosNetworkNamespace,
      type: TalosLinkStatusType,
      id: TalosKubeSpanLinkName,
    },
    initialResources: [
      {
        spec: {
          type: 'wireguard',
          operationalState: 'unknown',
        },
        metadata: {
          namespace: TalosNetworkNamespace,
          type: TalosLinkStatusType,
          id: TalosKubeSpanLinkName,
        },
      },
    ],
  }).handler,
  createWatchStreamHandler<ClusterMachineIdentitySpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: ClusterMachineIdentityType,
    },
    initialResources: machines.map(({ id, nodename }) => ({
      spec: {
        nodename,
      },
      metadata: {
        namespace: DefaultNamespace,
        type: ClusterMachineStatusType,
        id,
        labels: faker.helpers.weightedArrayElement<Record<string, string>>([
          { weight: 1, value: { [LabelControlPlaneRole]: '' } },
          { weight: 4, value: { [LabelWorkerRole]: '' } },
        ]),
      },
    })),
  }).handler,
  createWatchStreamHandler<PeerStatusSpec>({
    expectedOptions: {
      namespace: TalosKubeSpanNamespace,
      type: TalosKubeSpanPeerStatusType,
    },
    initialResources: ({ contextNode }) => {
      faker.seed((contextNode || '').split('').reduce((prev, curr) => prev + curr.charCodeAt(0), 0))

      return machines
        .filter((m) => m.id !== contextNode)
        .map(({ nodename }) => ({
          spec: {
            label: nodename,
            state: faker.helpers.weightedArrayElement([
              { value: 'up', weight: 4 },
              { value: 'down', weight: 1 },
            ]),
            endpoint: `${faker.internet.ipv4()}:${faker.internet.port()}`,
            lastUsedEndpoint: `${faker.internet.ipv4()}:${faker.internet.port()}`,
            receiveBytes: faker.number.int({ min: 0, max: 10_000_000_000 }),
            transmitBytes: faker.number.int({ min: 0, max: 10_000_000_000 }),
            lastHandshakeTime: faker.date.recent({ days: 1 }).toISOString(),
            lastEndpointChange: faker.date.recent({ days: 7 }).toISOString(),
          },
          metadata: {
            namespace: TalosKubeSpanNamespace,
            type: TalosKubeSpanPeerStatusType,
            id: faker.string.uuid(),
          },
        }))
    },
  }).handler,
]

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}

export const NotEnabled: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<LinkStatusSpec>({
        expectedOptions: {
          namespace: TalosNetworkNamespace,
          type: TalosLinkStatusType,
          id: TalosKubeSpanLinkName,
        },
      }).handler,
      ...handlers,
    )
  },
}
