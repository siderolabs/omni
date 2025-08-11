<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TNotification from '@/components/common/Notification/TNotification.vue'
import TShell from '@/components/common/Shell/TShell.vue'
import TSuspended from '@/components/common/Suspended/TSuspended.vue'
import TSideBar from '@/components/SideBar/TSideBar.vue'
import THeader from '@/components/THeader/THeader.vue'
import TModal from '@/components/TModal.vue'
import { suspended } from '@/methods'
import { notification } from '@/notification'
</script>

<template>
  <div class="root">
    <TModal />
    <THeader id="header" />
    <TShell class="shell">
      <template #menu>
        <TSideBar />
      </template>
      <template #content>
        <div class="flex h-full w-full flex-col gap-4">
          <TSuspended v-if="suspended" />
          <TNotification v-if="notification" v-bind="notification.props" />
          <router-view class="h-full w-full" />
        </div>
      </template>
    </TShell>
  </div>
</template>

<style scoped>
.root {
  @apply flex h-screen flex-col text-naturals-N11;
}

.shell {
  height: calc(100% - 52px);
}
</style>
