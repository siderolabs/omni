// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createSharedComposable } from '@vueuse/core'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

export const useFeatures = createSharedComposable(() =>
  useResourceWatch<FeaturesConfigSpec>({
    resource: {
      type: FeaturesConfigType,
      namespace: DefaultNamespace,
      id: FeaturesConfigID,
    },
    runtime: Runtime.Omni,
  }),
)

export function useIsEnterprise() {
  const { data } = useFeatures()

  return computed(() => data.value?.spec.is_enterprise_image_factory ?? false)
}
