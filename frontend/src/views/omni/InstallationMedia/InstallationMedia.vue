<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

onMounted(() => {
  // TODO: Only redirect if we don't have any media created yet, when the save/list APIs are in place
  router.replace({ name: 'InstallationMediaCreate' })
})
</script>

<template>Installation Media</template>
