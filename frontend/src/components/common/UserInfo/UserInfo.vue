<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useAuth0 } from '@auth0/auth0-vue'
import { computed, toRefs } from 'vue'

import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import { useLogout } from '@/methods/auth'
import { useIdentity } from '@/methods/identity'

const { identity: identityStorage } = useIdentity()
const logout = useLogout()

type Props = {
  withLogoutControls?: boolean
  size?: 'normal' | 'small'
  avatar?: string
  fullname?: string
  email?: string
}

const props = withDefaults(defineProps<Props>(), {
  size: 'normal',
})

const { avatar, fullname } = toRefs(props)

const auth0 = useAuth0()

const identity = computed(
  () => props.email || auth0?.user?.value?.email?.toLowerCase() || identityStorage.value,
)
const picture = computed(() => avatar.value ?? auth0?.user?.value?.picture)
const name = computed(() => fullname.value ?? auth0?.user?.value?.name)

const fontSize = computed(() => {
  if (props.size === 'small') {
    return { 'text-xs': true }
  }

  return ''
})

const imageSize = computed(() => {
  if (props.size === 'small') {
    return { 'w-8': true, 'h-8': true }
  }

  return { 'w-12': true, 'h-12': true }
})
</script>

<template>
  <div class="flex items-center gap-2" :class="fontSize">
    <img
      v-if="picture"
      class="rounded-full"
      :class="imageSize"
      :src="picture"
      referrerpolicy="no-referrer"
    />
    <div class="flex flex-1 flex-col truncate">
      <div class="truncate text-naturals-n13">{{ name }}</div>
      {{ identity }}
    </div>
    <TActionsBox v-if="withLogoutControls">
      <TActionsBoxItem @select="logout">Log Out</TActionsBoxItem>
    </TActionsBox>
  </div>
</template>
