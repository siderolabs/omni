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
@reference "../../../index.css";

.notification {
  @apply flex w-full items-center justify-between rounded border border-naturals-n5 bg-naturals-n0 px-6 py-4;
  min-height: 65px;
}
.notification--in-progress {
  @apply border-l-4 border-l-yellow-y3;
}
.notification--error {
  @apply border-l-4 border-l-red-r2;
}
.notification--success {
  @apply border-l-4 border-l-green-g1;
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
  @apply animate-spin text-yellow-y1;
}
.notification__error-icon {
  @apply text-red-r1;
}
.notification__success-icon {
  @apply text-green-g1;
}

.notification__title {
  @apply text-sm font-medium text-naturals-n14;
}
.notification__title--error {
  @apply text-red-r1;
}
.notification__description {
  @apply text-xs;
}
.notification__left-button {
  @apply mr-6 whitespace-nowrap;
}
</style>
