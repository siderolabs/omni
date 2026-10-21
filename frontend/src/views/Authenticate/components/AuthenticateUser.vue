<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/Button/TButton.vue'
import UserInfo from '@/components/UserInfo/UserInfo.vue'
import { useLogout } from '@/methods/auth'
import type { AuthUser } from '@/methods/useAuthProvider'

defineProps<{ user: AuthUser }>()

defineSlots<{
  /** The button that grants the access. */
  default(): unknown
}>()

const logout = useLogout()
</script>

<template>
  <div class="flex w-full flex-col gap-compact">
    <p>The keys are going to be issued for the user:</p>

    <UserInfo
      class="rounded-md bg-surface-inert px-base py-tight"
      :email="user.email"
      :avatar="user.picture"
      :fullname="user.name"
    />

    <div class="flex w-full flex-col gap-snug">
      <TButton variant="secondary" class="w-full" @click="logout">Switch User</TButton>
      <slot />
    </div>
  </div>
</template>
