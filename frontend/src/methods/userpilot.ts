// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'
import { Userpilot } from 'userpilot'
import { computed, ref, watch, watchEffect } from 'vue'

import { getNonce } from '@/methods'
import { currentUser } from '@/methods/auth'

import { useFeatures } from './features'

export function useUserpilot() {
  const { data } = useFeatures()
  const trackingState = useLocalStorage<boolean | null>('tracking', null)
  const userpilotInitialised = ref(false)

  const token = computed(() => data.value?.spec.user_pilot_settings?.app_token)
  const trackingFeatureEnabled = computed(() => !!token.value)
  const trackingPending = computed(() => trackingState.value === null)

  watchEffect((onCleanup) => {
    if (!token.value || !trackingState.value) return

    Userpilot.initialize(token.value, { nonce: getNonce() })
    userpilotInitialised.value = true

    onCleanup(() => Userpilot.destroy())
  })

  watch([currentUser, userpilotInitialised], ([user, initialised]) => {
    if (!user || !initialised) return

    Userpilot.identify(user.spec.user_id!, {
      role: user.spec.role!,
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
