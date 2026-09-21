<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import { useLogout } from '@/methods/auth'

const logout = useLogout()

const {
  size = 'normal',
  email,
  avatar,
  fullname,
} = defineProps<{
  withLogoutControls?: boolean
  size?: 'normal' | 'small'
  avatar?: string
  fullname?: string
  email?: string
}>()

const initials = computed(() => {
  const source = fullname?.trim() || email?.split('@')[0] || ''
  const words = source.split(/[\s._-]+/).filter(Boolean)
  const picked = words.length > 1 ? [words[0], words.at(-1)!] : words

  return picked
    .map((word) => {
      const [first] = new Intl.Segmenter().segment(word)
      return first.segment
    })
    .join('')
})
</script>

<template>
  <div class="flex items-center gap-2">
    <div class="overflow-hidden rounded-full" :class="size === 'small' ? 'size-8' : 'size-12'">
      <img v-if="avatar" class="size-full" :src="avatar" alt="" referrerpolicy="no-referrer" />

      <span
        v-else
        class="flex size-full items-center justify-center bg-naturals-n10 font-mono font-medium text-naturals-n0 uppercase"
        :class="size === 'small' ? 'text-xs' : 'text-base'"
        aria-hidden="true"
      >
        {{ initials }}
      </span>
    </div>

    <div class="flex grow flex-col overflow-hidden" :class="{ 'text-xs': size === 'small' }">
      <span class="truncate text-naturals-n13">{{ fullname }}</span>
      <span class="truncate">{{ email }}</span>
    </div>

    <div class="shrink-0">
      <TActionsBox v-if="withLogoutControls" aria-label="user actions">
        <TActionsBoxItem @select="logout">Log Out</TActionsBoxItem>
      </TActionsBox>
    </div>
  </div>
</template>
