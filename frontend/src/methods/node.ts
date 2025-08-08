// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
} from '@/api/resources'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import type { Ref } from 'vue'
import { computed, ref } from 'vue'
import Watch from '@/api/watch'

export const setupNodenameWatch = (id: string | string[]): Ref<string> => {
  const res: Ref<Resource<ClusterMachineStatusSpec> | undefined> = ref()

  const w = new Watch(res)

  w.setup({
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
      id: id as string,
    },
    runtime: Runtime.Omni,
  })

  return computed(() => {
    return (res.value?.metadata?.labels ?? {})[ClusterMachineStatusLabelNodeName] ?? ''
  })
}
