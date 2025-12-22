<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useAuth0 } from '@auth0/auth0-vue'
import { computed } from 'vue'

import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import { useLogout } from '@/methods/auth'
import { useIdentity } from '@/methods/identity'

const { identity: identityStorage } = useIdentity()
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

const auth0 = useAuth0()

const identity = computed(
  () => email || auth0?.user?.value?.email?.toLowerCase() || identityStorage.value,
)
const picture = computed(() => avatar || auth0?.user?.value?.picture)
const name = computed(() => fullname || auth0?.user?.value?.name)
</script>

<template>
  <div class="flex items-center gap-2" :class="{ 'text-xs': size === 'small' }">
    <img
      v-if="picture"
      class="rounded-full"
      :class="size === 'small' ? 'size-8' : 'size-12'"
      :src="picture"
      referrerpolicy="no-referrer"
    />
    <div class="flex grow flex-col overflow-hidden">
      <span class="truncate text-naturals-n13">{{ name }}</span>
      <span class="truncate">{{ identity }}</span>
    </div>
    <div class="shrink-0">
      <TActionsBox v-if="withLogoutControls">
        <TActionsBoxItem @select="logout">Log Out</TActionsBoxItem>
      </TActionsBox>
    </div>
  </div>
</template>
