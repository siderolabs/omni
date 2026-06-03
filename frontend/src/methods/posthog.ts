// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import posthog from 'posthog-js'
import { computed, ref, toValue, watchEffect } from 'vue'

import { useCurrentUser } from '@/methods/auth'

import { useFeatures } from './features'

export function usePostHog() {
  const { data: features } = useFeatures()
  const currentUser = useCurrentUser()
  const initialised = ref(false)

  const settings = computed(() => features.value?.spec.posthog_settings)

  watchEffect(() => {
    const apiKey = settings.value?.api_key
    const apiHost = settings.value?.api_host

    if (!apiKey || !apiHost || initialised.value) return

    posthog.init(apiKey, {
      api_host: apiHost,
      // Omni is served behind authentication, so only ever create person profiles
      // for identified users rather than anonymous traffic.
      person_profiles: 'identified_only',
    })

    initialised.value = true
  })

  watchEffect(() => {
    const user = toValue(currentUser)

    if (!user?.spec.user_id || !initialised.value) return

    posthog.identify(user.spec.user_id, {
      email: user.spec.identity,
      role: user.spec.role,
      created_at: user.metadata.created,
    })

    const companyID = features.value?.spec.account?.id
    if (companyID) {
      posthog.group('company', companyID)
    }
  })
}
