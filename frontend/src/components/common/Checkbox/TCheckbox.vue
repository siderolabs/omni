<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  checked?: boolean | number
  label?: string
  disabled?: boolean
  displayCheckedStatusWhenDisabled?: boolean
  icon?: IconType
}

withDefaults(defineProps<Props>(), {
  icon: 'check',
})
</script>

<template>
  <div
    class="checkbox-wrapper"
    :class="{ 'cursor-not-allowed': disabled, 'cursor-pointer': !disabled }"
    @click="(e) => (disabled ? e.stopImmediatePropagation() : null)"
  >
    <div class="checkbox" :class="{ checked: checked, disabled: disabled }">
      <TAnimation>
        <TIcon
          v-show="checked && (!disabled || displayCheckedStatusWhenDisabled)"
          class="checkbox-icon"
          :icon="icon"
        />
      </TAnimation>
      <input type="checkbox" hidden :value="checked" />
    </div>
    <span v-if="!!label" class="checkbox-label">{{ label }}</span>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.checkbox {
  @apply flex items-center justify-center border border-naturals-n7;
  width: 14px;
  height: 14px;
  border-radius: 2px;
}
.checkbox-wrapper {
  @apply flex items-center gap-2;
}
.checkbox-label {
  @apply block flex-1 truncate text-xs text-naturals-n11 select-none;
}
.checked {
  @apply border-primary-p6 bg-primary-p6;
}
.checkbox-icon {
  @apply fill-current text-primary-p3;
  width: 14px;
  height: 14px;
}

.disabled {
  @apply border-naturals-n5 bg-naturals-n4;
}
</style>
