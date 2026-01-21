<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useLocalStorage, useMediaQuery } from '@vueuse/core'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppToast from '@/components/common/AppToast/AppToast.vue'
import TSuspended from '@/components/common/Suspended/TSuspended.vue'
import UserConsent from '@/components/common/UserConsent/UserConsent.vue'
import THeader from '@/components/THeader/THeader.vue'
import TModal from '@/components/TModal.vue'
import { suspended } from '@/methods'
import { useRegisterAPIInterceptor } from '@/methods/interceptor'
import { useWatchKeyExpiry } from '@/methods/key'
import { useUserpilot } from '@/methods/userpilot'

useRegisterAPIInterceptor()
useWatchKeyExpiry()

const { trackingFeatureEnabled, trackingPending, enableTracking, disableTracking } = useUserpilot()

const isSidebarOpen = ref(false)
const router = useRouter()

router.afterEach(() => (isSidebarOpen.value = false))

const themesFeatureEnabled = useLocalStorage('_themes_enabled', false)
const themePreference = useLocalStorage<'dark' | 'light' | 'system'>('theme', 'system')
const isPreferredDark = useMediaQuery('(prefers-color-scheme: dark)')

const darkThemeEnabled = computed(() => {
  if (!themesFeatureEnabled.value) return true
  if (themePreference.value !== 'system') return themePreference.value

  return isPreferredDark.value ? 'dark' : 'light'
})
</script>

<template>
  <main class="flex h-screen flex-col" :class="{ dark: darkThemeEnabled }">
    <UserConsent
      v-if="trackingFeatureEnabled && trackingPending"
      @decline="disableTracking"
      @accept="enableTracking"
    />

    <AppToast />

    <THeader
      class="shrink-0"
      :sidebar-open="isSidebarOpen"
      @toggle-sidebar="isSidebarOpen = !isSidebarOpen"
    />

    <div class="relative flex grow overflow-hidden">
      <div
        class="pointer-events-none absolute inset-0 z-10 bg-black/75 opacity-0 transition-opacity duration-300"
        :class="{ 'max-md:pointer-events-auto max-md:opacity-100': isSidebarOpen }"
        @click="isSidebarOpen = false"
      ></div>

      <RouterView v-slot="{ Component }" name="sidebar">
        <component
          :is="Component"
          id="sidebar"
          class="top-0 bottom-0 left-0 z-10 w-64 shrink-0 transition-all duration-300 max-md:absolute"
          :class="{
            'max-md:pointer-events-none max-md:invisible max-md:-translate-x-full': !isSidebarOpen,
          }"
        />
      </RouterView>

      <div
        class="relative flex grow flex-col gap-4 overflow-auto"
        :class="{
          'max-md:pointer-events-none max-md:select-none': isSidebarOpen,
          'p-6': !$route.meta.disablePadding,
        }"
      >
        <TSuspended v-if="suspended" />
        <RouterView />
      </div>
    </div>
  </main>

  <TModal />
</template>
