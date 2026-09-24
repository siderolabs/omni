<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import ResourceDefinitionList from '@/views/Internals/ResourceDefinitionList.vue'

definePage({ name: 'InternalsOmniResources' })
</script>

<template>
  <ResourceDefinitionList :target="{ runtime: 'omni' }" />
</template>
