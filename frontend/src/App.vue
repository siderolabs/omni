<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="root">
    <t-modal/>
    <t-header id="header"/>
    <t-shell class="shell">
      <template #menu>
        <t-side-bar/>
      </template>
      <template #content>
        <div class="flex flex-col w-full h-full gap-4">
          <t-suspended v-if="suspended"/>
          <t-notification v-if="notification" v-bind="notification.props"/>
          <router-view class="w-full h-full"/>
        </div>
      </template>
    </t-shell>
  </div>
</template>

<script setup lang="ts">
import { notification } from "@/notification";
import { suspended } from "@/methods";

import THeader from "@/components/THeader/THeader.vue";
import TShell from "@/components/common/Shell/TShell.vue";
import TSideBar from "@/components/SideBar/TSideBar.vue";
import TModal from "@/components/TModal.vue";
import TSuspended from '@/components/common/Suspended/TSuspended.vue';
import TNotification from '@/components/common/Notification/TNotification.vue';
</script>

<style scoped>
.root {
  @apply text-naturals-N11 h-screen flex flex-col;
}

.shell {
  height: calc(100% - 52px);
}
</style>
