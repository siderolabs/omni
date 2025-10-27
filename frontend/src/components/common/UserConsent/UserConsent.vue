<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watch } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'
import { currentUser } from '@/methods/auth'
import { getUserPilotToken, initializeUserPilot } from '@/methods/features'
import { trackingState } from '@/methods/features'

const enableTracking = async () => {
  if (!currentUser.value) {
    return
  }

  trackingState.value = true

  await initializeUserPilot(currentUser.value)
}

const trackerToken = ref<string | undefined>()

const fetchTrackingToken = async () => {
  if (!currentUser.value) {
    return
  }

  trackerToken.value = await getUserPilotToken()
}

watch(() => currentUser.value, fetchTrackingToken)
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
              class="list-item-link cursor-pointer"
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
