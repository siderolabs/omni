<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

export type AlertType = 'error' | 'info' | 'success' | 'warn'

type Props = {
  type: AlertType
  title: string
  dismiss?: {
    name: string
    action: () => void
  }
}

defineProps<Props>()

const icons: Record<AlertType, IconType> = {
  error: 'error',
  info: 'info',
  success: 'check-in-circle',
  warn: 'warning',
}
</script>

<template>
  <div class="alert" :class="'alert-' + type">
    <div class="alert-box">
      <div id="icon" class="alert-icon-wrapper">
        <TIcon :icon="icons[type]" />
      </div>
      <div class="alert-info-wrapper">
        <h3 id="title">{{ title }}</h3>
        <div v-if="$slots.default" id="description">
          <p>
            <slot></slot>
          </p>
        </div>
      </div>
      <div v-if="dismiss" class="flex flex-1 justify-end pr-2">
        <TButton type="compact" class="notification-right-button" @click="dismiss?.action">{{
          dismiss.name
        }}</TButton>
      </div>
    </div>
  </div>
</template>

<style>
@reference "../index.css";

.alert {
  @apply rounded-md border border-l-4 border-naturals-n6 bg-naturals-n0 p-4;
}

.alert-box {
  @apply flex items-center;
}
.alert-icon-wrapper {
  @apply flex items-center justify-center;
}

.alert-info-wrapper {
  @apply ml-3 flex flex-col gap-2;
}

#title {
  @apply text-sm font-medium;
}

#description {
  @apply text-sm font-normal;
}

#icon > * {
  @apply h-5 w-5;
}

.alert-error {
  @apply border border-l-4 border-solid border-naturals-n5 border-l-red-r2;
}

.alert-error #title {
  @apply text-red-400;
}

.alert-error #icon {
  @apply text-red-400;
}

.alert-info {
  @apply border-l-blue-400;
}

.alert-info #title {
  @apply text-blue-400;
}

.alert-info #icon {
  @apply text-blue-400;
}

.alert-success {
  @apply border-l-green-g1;
}

.alert-success #title {
  @apply text-green-g1;
}

.alert-success #icon {
  @apply text-green-g1;
}

.alert-warn {
  border: 1px solid #272932;
  @apply border-l-4 border-l-yellow-y1;
}

.alert-warn #title {
  @apply text-yellow-y1;
}

.alert-warn #icon {
  @apply text-yellow-y1;
}
</style>
