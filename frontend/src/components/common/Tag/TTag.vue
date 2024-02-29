<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="tag">
    {{ name }}
  </div>
</template>

<script setup lang="ts">
import { toRefs } from "vue";

type Props = {
  name: string;
};

const props = defineProps<Props>();
const { name } = toRefs(props);
</script>

<style scoped>
.tag {
  @apply bg-naturals-N4 rounded text-xs text-naturals-N10;
  word-break: break-all;
  padding: 4px 6px 4px 6px;
}
</style>
