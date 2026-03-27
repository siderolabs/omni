<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { RoleNone } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import { useCurrentUser } from '@/methods/auth'
import HomeContent from '@/views/Home/HomeContent.vue'
import HomeNoAccess from '@/views/Home/HomeNoAccess.vue'

definePage({ name: 'Home' })

const currentUser = useCurrentUser()

const role = computed(() => currentUser.value?.spec.role ?? RoleNone)
</script>

<template>
  <PageContainer v-if="currentUser" class="h-full">
    <HomeNoAccess v-if="role === RoleNone" />
    <HomeContent v-else />
  </PageContainer>
</template>
