<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <button :disabled="disabled" class="icon-button" :class={danger}>
    <slot v-if="$slots.default"/>
    <t-icon
      v-else
      class="icon-button-icon"
      :class="iconClasses"
      :icon="icon"
    />
  </button>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { toRefs } from "vue";

type Props = {
  icon: IconType
  disabled?: boolean
  danger?: boolean
  iconClasses?: Record<string, boolean>
};

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
});
const { icon, disabled } = toRefs(props);
</script>

<style scoped>
button {
  @apply w-6 h-6;
}
.icon-button-icon {
  @apply cursor-pointer p-1 w-full h-full;
}

.icon-button {
  @apply text-naturals-N11
    hover:text-naturals-N13
    hover:bg-naturals-N7
    transition-all
    rounded
    duration-100
    disabled:text-naturals-N7
    disabled:cursor-not-allowed
    disabled:pointer-events-none;
}

.icon-button.danger {
  @apply text-red-R1;
}

.icon-button.highlighted {
  @apply bg-naturals-N7;
}
</style>
