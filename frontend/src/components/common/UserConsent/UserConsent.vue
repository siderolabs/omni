<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'

import TButton from '@/components/common/Button/TButton.vue'
import { currentUser } from '@/methods/auth'
import { getUserPilotToken, initializeUserPilot, trackingState } from '@/methods/features'

const enableTracking = async () => {
  if (!currentUser.value) {
    return
  }

  trackingState.value = true

  await initializeUserPilot(currentUser.value)
}

const trackerToken = computedAsync(async () => {
  if (!currentUser.value) {
    return
  }

  return await getUserPilotToken()
})
</script>

<template>
  <div v-if="trackingState === null && trackerToken">
    <div class="absolute top-0 left-0 z-50 flex h-screen w-screen flex-col justify-stretch">
      <div class="flex-1 bg-black opacity-25" />
      <div class="border-t border-primary-p3 bg-naturals-n6 p-4">
        <p class="text-lg text-naturals-n13">Cookies for a Better Experience</p>
        <div class="flex items-center justify-between">
          <p class="text-sm">
            We use cookies to improve our website's interface and onboarding experience. We don't
            use them for ads or share your data with third parties.
            <a
              class="link-primary"
              rel="noopener noreferrer"
              href="https://www.siderolabs.com/privacy-policy/"
              target="_blank"
            >
              Learn more
            </a>
          </p>
          <div class="flex gap-2">
            <TButton @click="trackingState = false">Decline</TButton>
            <TButton type="highlighted" @click="enableTracking">Accept</TButton>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
