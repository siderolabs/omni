<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <nav class="nav">
    <div class="nav-list">
      <t-menu-item
        v-for="item of items"
        :key="item.name"
        :route="item.route"
        :name="item.name"
        :icon="item.icon"
        :label="item.label"
        :label-color="item.labelColor"
      >
      </t-menu-item>
    </div>
  </nav>
</template>

<script setup lang="ts">
import TMenuItem from "@/components/common/MenuItem/TMenuItem.vue";
import { RouteLocationRaw } from "vue-router";
import { IconType } from "../common/Icon/TIcon.vue";

export type SideBarItem = {
  name: string,
  route: string | RouteLocationRaw,
  icon?: IconType,
  label?: string | number,
  labelColor?: string,
}

type Props = {
  items: SideBarItem[]
}

defineProps<Props>();
</script>

<style scoped>
.nav {
  @apply py-2;
}
.nav-list {
  @apply flex-col;
}
</style>
