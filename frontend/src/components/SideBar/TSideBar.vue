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
    <aside v-if="sidebar" class="flex max-w-64 flex-col border-r border-naturals-n4 bg-naturals-n1">
      <component :is="sidebar" v-if="sidebar" class="grow overflow-auto" />

      <UserInfo
        class="h-16 w-full shrink-0 border-t border-inherit px-2"
        with-logout-controls
        size="small"
      />
    </aside>
  </TAnimation>
</template>
