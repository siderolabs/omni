<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { RoleNone } from '@/api/resources'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import { currentUser } from '@/methods/auth'
import HomeContent from '@/views/omni/Home/HomeContent.vue'
import HomeNoAccess from '@/views/omni/Home/HomeNoAccess.vue'

definePage({ name: 'Home' })

const hasRoleNone = computed(() => {
  const role = currentUser.value?.spec?.role

  return !role || role === RoleNone
})
</script>

<template>
  <PageContainer>
    <HomeNoAccess v-if="hasRoleNone" />
    <HomeContent v-else />
  </PageContainer>
</template>
