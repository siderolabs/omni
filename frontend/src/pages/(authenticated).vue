<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AuthFlowQueryParam, FrontendAuthFlow, RedirectQueryParam } from '@/api/resources'
import { loadCurrentUser } from '@/methods/auth'
import { hasValidKeys } from '@/methods/key'
import { refreshTitle } from '@/methods/title'

const authReady = ref(false)
const router = useRouter()
const route = useRoute()

onBeforeMount(async () => {
  const authorized = await hasValidKeys()

  if (!authorized) {
    return router.replace({
      name: 'Authenticate',
      query: {
        [AuthFlowQueryParam]: FrontendAuthFlow,
        [RedirectQueryParam]: route.fullPath,
      },
    })
  }

  await loadCurrentUser()
  await refreshTitle()

  authReady.value = true
})
</script>

<template>
  <RouterView v-if="authReady" />
</template>
