<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AuthFlowQueryParam, FrontendAuthFlow, RedirectQueryParam } from '@/api/resources'
import { hasValidKeys } from '@/methods/key'
import { useDocumentTitle } from '@/methods/title'
import { useUserpilot } from '@/methods/userpilot'

useDocumentTitle()
useUserpilot()

const authorized = ref(false)
const router = useRouter()
const route = useRoute()

onBeforeMount(async () => {
  authorized.value = await hasValidKeys()

  if (!authorized.value) {
    return router.replace({
      name: 'Authenticate',
      query: {
        [AuthFlowQueryParam]: FrontendAuthFlow,
        [RedirectQueryParam]: route.fullPath,
      },
    })
  }
})
</script>

<template>
  <RouterView v-if="authorized" />
</template>
