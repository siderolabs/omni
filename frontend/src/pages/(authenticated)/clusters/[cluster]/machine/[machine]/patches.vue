<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import Patches from '@/views/Config/Patches.vue'

definePage({ name: 'NodePatches' })
</script>

<template>
  <PageContainer>
    <Patches :cluster="$route.params.cluster" :machine="$route.params.machine" />
  </PageContainer>
</template>
