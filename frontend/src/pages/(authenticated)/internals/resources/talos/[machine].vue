<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { useMachineName } from '@/methods/node'
import { useTitle } from '@/methods/title'

const route = useRoute()

const machineName = useMachineName(() =>
  'machine' in route.params ? route.params.machine : undefined,
)

useTitle(machineName)
</script>

<template>
  <RouterView />
</template>
