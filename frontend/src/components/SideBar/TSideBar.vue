<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'

import TAnimation from '@/components/common/Animation/TAnimation.vue'
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'

const route = useRoute()

const sidebar = shallowRef(route.meta.sidebar)

watch(
  () => [route.query, route.params],
  () => {
    sidebar.value = route.meta.sidebar
  },
)
</script>

<template>
  <TAnimation>
    <aside
      v-if="sidebar"
      class="relative h-full w-48 border-r border-naturals-n4 bg-naturals-n1 lg:w-64"
    >
      <div class="flex h-full flex-col">
        <component :is="sidebar" v-if="sidebar" class="flex-1 overflow-y-auto" />
        <UserInfo
          class="h-16 w-full border-t border-naturals-n4 px-2"
          with-logout-controls
          size="small"
        />
      </div>
    </aside>
  </TAnimation>
</template>
