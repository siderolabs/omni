<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import ResourceEntryList from '@/views/Internals/ResourceEntryList.vue'

definePage({ name: 'InternalsTalosResource' })
</script>

<template>
  <ResourceEntryList
    :target="{ runtime: 'talos', machine: $route.params.machine }"
    :type="$route.params.type"
  />
</template>
