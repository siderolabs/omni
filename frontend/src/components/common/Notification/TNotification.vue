<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { watch } from 'vue'
import { useRoute } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  type?: 'error' | 'in-progress' | 'success' | 'info'
  title?: string
  body?: string
  abort?: () => void
  isButtonHidden?: boolean
  buttonTitle?: 'Dismiss' | 'Abort'
  onLeftButtonClick?: () => void
  onRightButtonClick?: () => void
  close?: () => void
}

const props = withDefaults(defineProps<Props>(), {
  type: 'in-progress',
  buttonTitle: 'Dismiss',
})

const route = useRoute()

watch(
  () => route.path,
  () => {
    if (props.close) props.close()
  },
)
</script>

<template>
  <div class="notification" :class="'notification--' + type">
    <div class="notification__wrapper">
      <TIcon
        v-if="type === 'in-progress'"
        class="notification__icon notification__in-progress-icon"
        icon="loading"
      />
      <TIcon
        v-if="type === 'error'"
        class="notification__icon notification__error-icon"
        icon="error"
      />
      <TIcon
        v-if="type === 'success'"
        class="notification__icon notification__success-icon"
        icon="check-in-circle-classic"
      />
      <div class="notification__content-box">
        <h2 class="notification__title" :class="type === 'error' && 'notification__title--error'">
          {{ title }}
        </h2>
        <p v-if="body" class="notification__description">{{ body }}</p>
      </div>
    </div>
    <div class="notification__button-box">
      <!-- Todo -->
      <!-- <t-button
        type="subtle"
        icon="arrow-right"
        iconPosition="right"
        class="notification__left-button"
        @click="$emit('onLeftButtonClick')"
      >
        View details</t-button
      > -->
      <TButton
        v-if="!isButtonHidden"
        type="primary"
        class="notification__right-button"
        @click="
          () => {
            abort ? abort() : close ? close() : null
          }
        "
        >{{ buttonTitle }}</TButton
      >
    </div>
  </div>
</template>

<style scoped>
.notification {
  @apply flex w-full items-center justify-between rounded border border-naturals-N5 bg-naturals-N0 px-6 py-4;
  min-height: 65px;
}
.notification--in-progress {
  @apply border-l-4;
  --tw-border-opacity: 1;
  border-left-color: rgba(169, 120, 9, var(--tw-border-opacity));
}
.notification--error {
  @apply border-l-4;
  --tw-border-opacity: 1;
  border-left-color: rgba(110, 47, 48, var(--tw-border-opacity));
}
.notification--success {
  @apply border-l-4;
  border-left-color: #69c297;
}
.notification__wrapper {
  @apply flex items-center;
}
.notification__button-box {
  @apply flex items-center justify-end;
}
.notification__icon {
  @apply mr-5 fill-current;
  width: 20px;
  height: 20px;
}
.notification__in-progress-icon {
  @apply animate-spin text-yellow-Y1;
}
.notification__error-icon {
  @apply text-red-R1;
}
.notification__success-icon {
  @apply text-green-G1;
}

.notification__title {
  @apply text-sm font-medium text-naturals-N14;
}
.notification__title--error {
  @apply text-red-R1;
}
.notification__description {
  @apply text-xs;
}
.notification__left-button {
  @apply mr-6 whitespace-nowrap;
}
</style>
