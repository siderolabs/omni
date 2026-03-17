<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import Patches from '@/views/cluster/Config/Patches.vue'

definePage({ name: 'MachineConfigPatches' })
</script>

<template>
  <PageContainer>
    <Patches :machine="$route.params.machine" />
  </PageContainer>
</template>
