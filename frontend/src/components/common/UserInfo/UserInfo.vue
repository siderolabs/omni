<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useAuth0 } from '@auth0/auth0-vue'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import { AuthType, authType } from '@/methods'
import { currentUser } from '@/methods/auth'
import { resetKeys } from '@/methods/key'

type Props = {
  withLogoutControls?: boolean
  size?: 'normal' | 'small'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'normal',
})

const auth0 = useAuth0()

const route = useRoute()

const identity = computed(
  () =>
    route.query.identity ||
    auth0?.user?.value?.email?.toLowerCase() ||
    window.localStorage.getItem('identity'),
)
const avatar = computed(
  () => route.query.avatar || auth0?.user?.value?.picture || window.localStorage.getItem('avatar'),
)
const fullname = computed(
  () => route.query.fullname || auth0?.user?.value?.name || window.localStorage.getItem('fullname'),
)

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

const doLogout = async () => {
  await auth0?.logout({ logoutParams: { returnTo: window.location.origin } })

  resetKeys()

  currentUser.value = undefined

  if (authType.value === AuthType.SAML) {
    location.reload()
  }
}
</script>

<template>
  <div class="flex items-center gap-2" :class="fontSize">
    <img
      v-if="avatar"
      class="rounded-full"
      :class="imageSize"
      :src="avatar as string"
      referrerpolicy="no-referrer"
    />
    <div class="flex flex-1 flex-col truncate">
      <div class="truncate text-naturals-N13">{{ fullname }}</div>
      {{ identity }}
    </div>
    <TActionsBox v-if="withLogoutControls" placement="top">
      <div @click="doLogout">
        <div class="cursor-pointer px-4 py-2 hover:text-naturals-N12">Log Out</div>
      </div>
    </TActionsBox>
  </div>
</template>
