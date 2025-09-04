<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import AppToast from '@/components/common/AppToast/AppToast.vue'
import TSuspended from '@/components/common/Suspended/TSuspended.vue'
import TSideBar from '@/components/SideBar/TSideBar.vue'
import THeader from '@/components/THeader/THeader.vue'
import TModal from '@/components/TModal.vue'
import { suspended } from '@/methods'

const isSidebarOpen = ref(false)
const router = useRouter()

router.afterEach(() => (isSidebarOpen.value = false))
</script>

<template>
  <main class="flex h-screen flex-col">
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
      <TSideBar
        id="sidebar"
        class="top-0 bottom-0 left-0 z-10 transition-all duration-300 max-md:absolute"
        :class="{
          'max-md:pointer-events-none max-md:invisible max-md:-translate-x-full': !isSidebarOpen,
        }"
      />

      <div
        class="relative flex grow flex-col gap-4 overflow-auto p-6"
        :class="{ 'max-md:pointer-events-none max-md:select-none': isSidebarOpen }"
      >
        <TSuspended v-if="suspended" />
        <RouterView />
      </div>
    </div>
  </main>

  <TModal />
</template>
