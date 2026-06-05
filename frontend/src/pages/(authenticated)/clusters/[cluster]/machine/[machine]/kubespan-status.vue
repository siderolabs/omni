<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import KubeSpanStatus from '@/views/KubeSpanStatus/KubeSpanStatus.vue'

definePage({ name: 'NodeKubeSpanStatus' })
</script>

<template>
  <KubeSpanStatus :cluster-id="$route.params.cluster" :machine-id="$route.params.machine" />
</template>
