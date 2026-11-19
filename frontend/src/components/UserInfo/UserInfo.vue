<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import { useInitials } from '@/components/UserInfo/lib/useInitials'
import { useLogout } from '@/methods/auth'

const logout = useLogout()

const {
  size = 'normal',
  email = '',
  avatar,
  fullname,
} = defineProps<{
  withLogoutControls?: boolean
  size?: 'normal' | 'small'
  avatar?: string
  fullname?: string
  email?: string
}>()

const initials = useInitials(() => fullname?.trim() || email)
</script>

<template>
  <div class="flex items-center gap-tight">
    <div
      class="shrink-0 overflow-hidden rounded-full"
      :class="size === 'small' ? 'size-8' : 'size-12'"
    >
      <img v-if="avatar" class="size-full" :src="avatar" alt="" referrerpolicy="no-referrer" />

      <span
        v-else
        class="flex size-full items-center justify-center bg-surface-inverse font-mono font-medium text-content-inverse uppercase"
        :class="size === 'small' ? 'text-xs' : 'text-base'"
        aria-hidden="true"
      >
        {{ initials }}
      </span>
    </div>

    <div class="flex grow flex-col overflow-hidden" :class="{ 'text-xs': size === 'small' }">
      <span v-if="fullname" class="truncate text-content-default">{{ fullname }}</span>
      <span class="truncate">{{ email }}</span>
    </div>

    <TActionsBox v-if="withLogoutControls" aria-label="user actions" class="shrink-0">
      <TActionsBoxItem @select="logout">Log Out</TActionsBoxItem>
    </TActionsBox>
  </div>
</template>
