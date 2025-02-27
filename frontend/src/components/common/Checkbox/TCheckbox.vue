<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="checkbox-wrapper" :class="{ 'cursor-not-allowed': disabled, 'cursor-pointer': !disabled }" @click="(e) => disabled ? e.stopImmediatePropagation() : null">
    <div class="checkbox" :class="{ checked: checked, disabled: disabled }">
      <t-animation>
        <t-icon class="checkbox-icon" :icon="icon" v-show="checked && (!disabled || displayCheckedStatusWhenDisabled)" />
      </t-animation>
      <input type="checkbox" hidden :value="checked" />
    </div>
    <span v-if="!!label" class="checkbox-label">{{ label }}</span>
  </div>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import TAnimation from "@/components/common/Animation/TAnimation.vue";

type Props = {
  checked?: boolean | number;
  label?: string;
  disabled?: boolean;
  displayCheckedStatusWhenDisabled?: boolean;
  icon?: IconType
};

withDefaults(
  defineProps<Props>(),
  {
    icon: "check"
  },
);
</script>

<style scoped>
.checkbox {
  @apply border border-naturals-N7 flex items-center justify-center;
  width: 14px;
  height: 14px;
  border-radius: 2px;
}
.checkbox-wrapper {
  @apply flex items-center gap-2;
}
.checkbox-label {
  @apply text-xs text-naturals-N11 block select-none truncate flex-1;
}
.checked {
  @apply border-primary-P6 bg-primary-P6;
}
.checkbox-icon {
  @apply fill-current text-primary-P3;
  width: 14px;
  height: 14px;
}

.disabled {
  @apply border-naturals-N5 bg-naturals-N4;
}
</style>
