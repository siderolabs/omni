<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="alert" :class="'alert-' + type">
    <div class="alert-box">
      <div class="alert-icon-wrapper" id="icon">
        <t-icon :icon="icons[type]"/>
      </div>
      <div class="alert-info-wrapper">
        <h3 id="title">{{ title }}</h3>
        <div id="description" v-if="$slots.default">
          <p>
            <slot></slot>
          </p>
        </div>
      </div>
      <div class="flex-1 flex justify-end pr-2" v-if="dismiss">
        <t-button
          type="compact"
          class="notification-right-button"
          @click="dismiss?.action"
          >{{ dismiss.name }}</t-button
        >
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";

import TButton from "@/components/common/Button/TButton.vue";

export type AlertType = "error" | "info" | "success" | "warn";

type Props = {
  type: AlertType;
  title: string;
  dismiss?: {
    name: string;
    action: () => void;
  };
};

defineProps<Props>();

const icons: Record<AlertType, IconType> = {
  "error": "error",
  "info": "info",
  "success": "check-in-circle",
  "warn": "warning",
};
</script>

<style>
.alert {
  @apply p-4 rounded-md bg-naturals-N0 border border-naturals-N6 border-l-4;
}

.alert-box {
  @apply flex items-center;
}
.alert-icon-wrapper {
  @apply flex justify-center items-center;
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
  @apply w-5 h-5;
}

.alert-error {
  border: 1px solid #272932;
  border-left-width: 4px;
  border-left-color: #6e2f30;
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
  @apply border-l-green-G1;
}

.alert-success #title {
  @apply text-green-G1;
}

.alert-success #icon {
  @apply text-green-G1;
}

.alert-warn {
  border: 1px solid #272932;
  @apply border-l-yellow-Y1 border-l-4;
}

.alert-warn #title {
  @apply text-yellow-Y1;
}

.alert-warn #icon {
  @apply text-yellow-Y1;
}
</style>
