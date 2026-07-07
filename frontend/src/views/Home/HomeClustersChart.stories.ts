// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { type ClusterStatusMetricsSpec, ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb.ts'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  EphemeralNamespace,
} from '@/api/resources.ts'

import HomeClustersChart from './HomeClustersChart.vue'

const meta: Meta<typeof HomeClustersChart> = {
  component: HomeClustersChart,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterStatusMetricsSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: ClusterStatusMetricsType,
            id: ClusterStatusMetricsID,
          },
          initialResources: [
            {
              spec: {
                not_ready_count: 3,
                phases: {
                  [ClusterStatusSpecPhase.DESTROYING]: 1,
                  [ClusterStatusSpecPhase.RUNNING]: 24,
                  [ClusterStatusSpecPhase.SCALING_DOWN]: 2,
                  [ClusterStatusSpecPhase.SCALING_UP]: 3,
                },
              },
              metadata: {
                namespace: EphemeralNamespace,
                type: ClusterStatusMetricsType,
                id: ClusterStatusMetricsID,
              },
            },
          ],
        }).handler,
      ],
    },
  },
} satisfies Story
