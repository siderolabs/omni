<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import ClusterManifestsStatus from '@/views/Clusters/ClusterManifestsStatus.vue'

definePage({ name: 'ClusterManifestsStatus' })
</script>

<template>
  <ClusterManifestsStatus :cluster="$route.params.cluster" />
</template>
