// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb.ts'
import {
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources.ts'

import HomeMachinesChart from './HomeMachinesChart.vue'

const meta: Meta = {
  component: HomeMachinesChart,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<MachineStatusMetricsSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: MachineStatusMetricsType,
            id: MachineStatusMetricsID,
          },
          initialResources: [
            {
              spec: {
                allocated_machines_count: 34,
                connected_machines_count: 40,
                pending_machines_count: 2,
                registered_machines_count: 42,
              },
              metadata: {
                namespace: EphemeralNamespace,
                type: MachineStatusMetricsType,
                id: MachineStatusMetricsID,
              },
            },
          ],
        }).handler,
      ],
    },
  },
} satisfies Story
