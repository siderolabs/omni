<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { RoleNone } from '@/api/resources'
import { currentUser } from '@/methods/auth'
import HomeContent from '@/views/omni/Home/HomeContent.vue'
import HomeNoAccess from '@/views/omni/Home/HomeNoAccess.vue'

const hasRoleNone = computed(() => {
  const role = currentUser.value?.spec?.role

  return !role || role === RoleNone
})
</script>

<template>
  <HomeNoAccess v-if="hasRoleNone" />
  <HomeContent v-else />
</template>
