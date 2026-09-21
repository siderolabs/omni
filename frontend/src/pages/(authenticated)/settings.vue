<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useTitle } from '@/methods/title'

definePage({
  name: 'Settings',
  redirect: {
    name: 'Users',
  },
})

useTitle('Settings')
</script>

<template>
  <RouterView />
</template>
