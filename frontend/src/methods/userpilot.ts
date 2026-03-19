// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { Userpilot } from 'userpilot'
import { computed, ref, toValue, watchEffect } from 'vue'

import { getNonce } from '@/methods'
import { useCurrentUser } from '@/methods/auth'

import { useFeatures } from './features'

export function useUserpilot() {
  const { data: features } = useFeatures()
  const currentUser = useCurrentUser()
  const userpilotInitialised = ref(false)

  const userPilotAppToken = computed(() => features.value?.spec.user_pilot_settings?.app_token)

  watchEffect((onCleanup) => {
    if (!userPilotAppToken.value) return

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
}
