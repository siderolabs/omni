<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { useTitle } from '@/methods/title'
import PatchEdit from '@/views/Config/PatchEdit.vue'

definePage({ name: 'ClusterPatchEdit' })

const route = useRoute()

useTitle(() => ['Config Patches', route.params.patch])
</script>

<template>
  <PatchEdit
    :patch-id="$route.params.patch"
    :cluster-id="$route.params.cluster"
    :back="{ name: 'ClusterConfigPatches' }"
  />
</template>
