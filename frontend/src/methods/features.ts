// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

export function useFeatures() {
  return useResourceWatch<FeaturesConfigSpec>({
    resource: {
      type: FeaturesConfigType,
      namespace: DefaultNamespace,
      id: FeaturesConfigID,
    },
    runtime: Runtime.Omni,
  })
}
