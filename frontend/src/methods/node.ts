// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
} from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

export const setupNodenameWatch = (id: string) => {
  const { data } = useResourceWatch<ClusterMachineStatusSpec>({
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
      id,
    },
    runtime: Runtime.Omni,
  })

  return computed(() => data.value?.metadata.labels?.[ClusterMachineStatusLabelNodeName] ?? '')
}
