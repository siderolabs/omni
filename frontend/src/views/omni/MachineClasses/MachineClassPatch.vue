<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <patches :machine-class="$route.params.classname as string"/>
</template>

<script setup lang="ts">
import Patches from '@/views/cluster/Config/Patches.vue';
</script>
