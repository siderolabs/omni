<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import ClusterSecurity from '@/views/ClusterSecurity/ClusterSecurity.vue'

definePage({ name: 'ClusterSecurity' })
</script>

<template>
  <PageContainer>
    <ClusterSecurity :cluster-id="$route.params.cluster" />
  </PageContainer>
</template>
