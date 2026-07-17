// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, effectScope } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, FeaturesConfigID, FeaturesConfigType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

let features: ReturnType<typeof initFeatures> | undefined

export function useFeatures() {
  features ||= initFeatures()

  return features
}

export function useIsEnterprise() {
  const { data } = useFeatures()

  return computed(() => data.value?.spec.is_enterprise_image_factory ?? false)
}

function initFeatures() {
  return effectScope(true).run(() => {
    return useResourceWatch<FeaturesConfigSpec>({
      resource: {
        type: FeaturesConfigType,
        namespace: DefaultNamespace,
        id: FeaturesConfigID,
      },
      runtime: Runtime.Omni,
    })
  })!
}
