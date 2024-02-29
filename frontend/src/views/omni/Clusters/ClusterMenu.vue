<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="menu flex items-center">
    <div class="flex flex-1 flex-col">
      <div class="menu-amount-box flex-1">
        <span class="menu-amount-box-light">{{ controlPlaneCount }},</span>
        <span class="menu-amount-box-light">{{ workersCount }}</span> selected
      </div>
      <div v-if="warning" class="text-yellow-Y1 text-xs">{{ warning }}</div>
    </div>
    <t-button v-if="onReset" @click="onReset" type="secondary">
      Cancel
    </t-button>
    <t-button iconPosition="left" @click="onSubmit" type="highlighted" :disabled="disabled">
      {{ action }}
    </t-button>
  </div>
</template>

<script setup lang="ts">
import pluralize from "pluralize";
import { computed, toRefs } from "vue";

import TButton from "@/components/common/Button/TButton.vue";

type Props = {
  action: string,
  controlPlanes?: number | string,
  workers?: number | string,
  onSubmit: () => void,
  onReset?: () => void,
  warning?: string
  disabled?: boolean
}


const props = withDefaults(defineProps<Props>(), {
  disabled: false
});

const { controlPlanes, workers } = toRefs(props);

const controlPlaneCount = computed(() => {
  if (typeof controlPlanes?.value === "number") {
    return `${controlPlanes.value} Control ${pluralize("Plane", controlPlanes.value as number, false)}`;
  }

  return `Control Planes: ${controlPlanes?.value}`;
});

const workersCount = computed(() => {
  if (typeof workers?.value === "number") {
    return `${workers.value} ${pluralize("Worker", workers.value as number, false)}`;
  }

  return `Workers: ${workers?.value}`;
});
</script>

<style scoped>
.menu {
  @apply flex gap-4;
}
.menu-amount-box {
  @apply text-xs text-naturals-N8 flex items-center;
}
.menu-amount-box-light {
  @apply text-naturals-N13 mr-1;
}
.menu-buttons-box {
  @apply flex items-center;
}
.menu-exit-button {
  @apply fill-current text-naturals-N7 cursor-pointer transition-colors hover:text-naturals-N8 w-6 h-6;
}
</style>
