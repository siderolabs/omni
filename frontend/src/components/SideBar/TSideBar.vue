<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'
</script>

<template>
  <aside class="flex max-w-64 flex-col border-r border-naturals-n4 bg-naturals-n1">
    <RouterView name="sidebar" class="grow overflow-auto" />

    <UserInfo
      class="h-16 w-full shrink-0 border-t border-inherit px-2"
      with-logout-controls
      size="small"
    />
  </aside>
</template>
