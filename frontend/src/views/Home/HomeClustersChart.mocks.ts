// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'

import { type ClusterStatusMetricsSpec, ClusterStatusSpecPhase } from '@/api/omni/specs/omni.pb'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  EphemeralNamespace,
} from '@/api/resources'

export const handlers = [
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
]
