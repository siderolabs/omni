// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import {
  type ClusterStatusSpec,
  ClusterStatusSpecPhase,
  MachineStatusSnapshotSpecPowerStage,
} from '@/api/omni/specs/omni.pb'
import type { PermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterPermissionsType,
  ClusterStatusType,
  DefaultNamespace,
  LabelCluster,
  MachineStatusLinkType,
  MetricsNamespace,
  PermissionsID,
  PermissionsType,
  VirtualNamespace,
} from '@/api/resources'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'
import { handlers as homeClustersChartHandlers } from '@/views/Home/HomeClustersChart.mocks'
import { handlers as homeGeneralInfoHandlers } from '@/views/Home/HomeGeneralInformation.mocks'
import { handlers as homeMachinesChartHandlers } from '@/views/Home/HomeMachinesChart.mocks'
import { handlers as homeOngoingOperationsHandlers } from '@/views/Home/HomeOngoingOperations.mocks'

import HomeContent from './HomeContent.vue'

faker.seed(0)

const meta: Meta<typeof HomeContent> = {
  component: HomeContent,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.post<never, GetRequest, GetResponse>(
        '/omni.resources.ResourceService/Get',
        async ({ request }) => {
          const { id, type, namespace } = await request.clone().json()

          if (id !== PermissionsID || type !== PermissionsType || namespace !== VirtualNamespace) {
            return
          }

          return HttpResponse.json({
            body: JSON.stringify({
              metadata: {
                namespace: VirtualNamespace,
                type: ClusterPermissionsType,
                id: PermissionsID,
              },
              spec: {
                can_read_clusters: true,
                can_read_machines: true,
                can_read_join_tokens: true,
                can_read_audit_log: true,
              },
            } as Resource<PermissionsSpec>),
          })
        },
      ),
      createWatchStreamHandler<ClusterStatusSpec>({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: ClusterStatusType,
        },
        initialResources: faker.helpers.multiple(
          () => ({
            spec: {
              phase: faker.helpers.enumValue(ClusterStatusSpecPhase),
              ready: faker.datatype.boolean(),
              machines: {
                total: faker.number.int({ min: 3, max: 50 }),
              },
            },
            metadata: {
              namespace: DefaultNamespace,
              type: ClusterStatusType,
              id: faker.hacker.noun(),
            },
          }),
          { count: 10 },
        ),
      }).handler,
      createWatchStreamHandler<MachineStatusLinkSpec>({
        expectedOptions: {
          namespace: MetricsNamespace,
          type: MachineStatusLinkType,
        },
        initialResources: faker.helpers.multiple(
          () => ({
            spec: {
              message_status: {
                network: {
                  hostname: faker.internet.domainWord(),
                },
              },
              tearing_down: faker.datatype.boolean(),
              snapshot: {
                machine_status: {
                  stage: faker.helpers.enumValue(MachineStatusEventMachineStage),
                },
                power_stage: faker.helpers.enumValue(MachineStatusSnapshotSpecPowerStage),
              },
            },
            metadata: {
              namespace: MetricsNamespace,
              type: MachineStatusLinkType,
              id: faker.string.uuid(),
              labels: {
                ...faker.helpers.maybe(() => ({
                  [LabelCluster]: faker.hacker.noun(),
                })),
              },
            },
          }),
          { count: 50 },
        ),
      }).handler,
      ...homeGeneralInfoHandlers,
      ...homeClustersChartHandlers,
      ...homeMachinesChartHandlers,
      ...homeOngoingOperationsHandlers,
    )
  },
}
