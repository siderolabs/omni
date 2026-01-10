// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'
import { Userpilot } from 'userpilot'
import { computed, ref, toValue, watchEffect } from 'vue'

import { getNonce } from '@/methods'
import { currentUser } from '@/methods/auth'

import { useFeatures } from './features'

export function useUserpilot() {
  const { data: features } = useFeatures()
  const trackingState = useLocalStorage<boolean | null>('tracking', null)
  const userpilotInitialised = ref(false)

  const userPilotAppToken = computed(() => features.value?.spec.user_pilot_settings?.app_token)
  const trackingFeatureEnabled = computed(() => !!userPilotAppToken.value)
  const trackingPending = computed(() => trackingState.value === null)

  watchEffect((onCleanup) => {
    if (!userPilotAppToken.value || !trackingState.value) return

    Userpilot.initialize(userPilotAppToken.value, { nonce: getNonce() })
    userpilotInitialised.value = true

    onCleanup(() => Userpilot.destroy())
  })

  watchEffect(() => {
    const user = toValue(currentUser)

    if (!user?.spec.user_id || !userpilotInitialised.value) return

    Userpilot.identify(user.spec.user_id, {
      role: user.spec.role,
      email: user.spec.identity,
      created_at: user.metadata.created,

      company: {
        id: features.value?.spec.account?.id,
      },
    })
  })

  return {
    trackingFeatureEnabled,
    trackingPending,
    enableTracking() {
      trackingState.value = true
    },
    disableTracking() {
      trackingState.value = false
    },
  }
}
