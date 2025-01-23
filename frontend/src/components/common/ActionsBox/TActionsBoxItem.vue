<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="action-item" :class="{'action-item-danger': danger}">
    <t-icon class="action-item-icon" :icon="icon"/>
    <span class="action-item-text">
        <slot/>
    </span>
  </div>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";

type Props = {
  icon: IconType;
  danger?: boolean;
};

withDefaults(defineProps<Props>(), {
  danger: false,
});
</script>

<style scoped>
.action-item {
  @apply flex items-center cursor-pointer w-full px-3 py-2 gap-2;
}

.action-item:hover > * {
  @apply text-naturals-N12;
}

.action-item-icon {
  @apply w-4 h-4 transition-colors;
}

.action-item-text {
  @apply text-xs transition-colors;
}

.action-item-danger > * {
  @apply text-red-R1;
}

.action-item-danger:hover > * {
  @apply text-primary-P1;
}

</style>
