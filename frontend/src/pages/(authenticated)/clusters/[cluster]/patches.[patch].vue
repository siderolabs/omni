<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PatchEdit from '@/views/cluster/Config/PatchEdit.vue'

definePage({ name: 'ClusterPatchEdit' })
</script>

<template>
  <PatchEdit
    :patch-id="$route.params.patch"
    :cluster-id="$route.params.cluster"
    :back="{ name: 'ClusterConfigPatches' }"
  />
</template>
