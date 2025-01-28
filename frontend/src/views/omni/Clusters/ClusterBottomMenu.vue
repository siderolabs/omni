<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-animation>
    <div class="menu flex items-center">
      <div class="flex flex-col">
        <div class="menu__amount-box flex-1">
          <span class="menu__amount-box--light">{{ controlPlanes }}</span>
          <span class="menu__amount-box--light">Control {{ pluralize('Plane', controlPlanes, false) }},</span>
          <span class="menu__amount-box--light">{{ workers }}</span>
          <span class="menu__amount-box--light">{{ pluralize('Worker', workers, false) }}</span> selected
        </div>
        <div v-if="warning" class="text-red-R1">{{ warning }}</div>
      </div>
      <t-button iconPosition="left" @click="onSubmit">
        {{ action }}
      </t-button>
      <t-icon class="menu__exit-button" icon="close" @click="onReset"/>
    </div>
  </t-animation>
</template>

<script setup lang="ts">
import pluralize from "pluralize";

import TAnimation from "@/components/common/Animation/TAnimation.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";

defineProps<{
  action: string,
  controlPlanes?: number,
  workers?: number,
  onSubmit: () => void,
  onReset?: () => void,
  warning?: string
}>();
</script>

<style scoped>
.menu {
  @apply bg-naturals-N3 p-5 fixed rounded z-20 flex w-full gap-4;
  width: 452px;
  min-height: 56px;
  bottom: 32px;
  left: calc(50% - 240px);
}
.menu__amount-box {
  @apply text-xs text-naturals-N8 flex items-center;
}
.menu__amount-box--light {
  @apply text-naturals-N13 mr-1;
}
.menu__buttons-box {
  @apply flex items-center;
}
.menu__exit-button {
  @apply fill-current text-naturals-N7 cursor-pointer transition-colors hover:text-naturals-N8 w-6 h-6;
}
</style>
