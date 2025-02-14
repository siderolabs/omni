<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-animation>
    <aside class="sidebar" v-if="sidebar">
      <div class="flex flex-col h-full">
        <component class="flex-1 overflow-y-auto" :is="sidebar" v-if="sidebar"/>
        <user-info class="w-full h-16 px-2 border-t border-naturals-N4" with-logout-controls size="small"/>
      </div>
    </aside>
  </t-animation>
</template>

<script setup lang="ts">
import { shallowRef, watch } from "vue";
import { useRoute } from "vue-router";
import { getSidebar } from "@/router";

import TAnimation from "@/components/common/Animation/TAnimation.vue";
import UserInfo from "@/components/common/UserInfo/UserInfo.vue";

const route = useRoute();

const sidebar = shallowRef(getSidebar(route));

watch(
  () => [route.query, route.params],
  () => {
    sidebar.value = getSidebar(route);
  }
);
</script>

<style scoped>
.sidebar {
  @apply bg-naturals-N1 relative w-48 lg:w-64 h-full border-r border-naturals-N4;
}
</style>
