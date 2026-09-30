<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/Button/TButton.vue'
import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'

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
  <div
    class="rounded-md border p-4"
    :class="{
      'border-l-3 border-status-danger-border border-l-status-danger-fill bg-status-danger-surface':
        type === 'error',
      'border-status-info-border bg-status-info-surface': type === 'info',
      'border-l-3 border-status-success-border border-l-status-success-fill bg-status-success-surface':
        type === 'success',
      'border-l-3 border-status-warning-border border-l-status-warning-fill bg-status-warning-surface':
        type === 'warn',
    }"
  >
    <div class="flex items-center">
      <div
        class="flex items-center justify-center"
        :class="{
          'text-status-danger-text': type === 'error',
          'text-status-info-text': type === 'info',
          'text-status-success-text': type === 'success',
          'text-status-warning-text': type === 'warn',
        }"
      >
        <TIcon :icon="icons[type]" class="size-5" />
      </div>
      <div class="ml-3 flex flex-col gap-2">
        <h3
          class="text-sm font-medium"
          :class="{
            'text-status-danger-text': type === 'error',
            'text-status-info-text': type === 'info',
            'text-status-success-text': type === 'success',
            'text-status-warning-text': type === 'warn',
          }"
        >
          {{ title }}
        </h3>
        <div
          v-if="$slots.default"
          class="text-sm font-normal whitespace-pre-wrap text-content-default"
        >
          <p>
            <slot></slot>
          </p>
        </div>
      </div>
      <div v-if="dismiss" class="flex flex-1 justify-end pr-2">
        <TButton size="sm" class="notification-right-button" @click="dismiss?.action">
          {{ dismiss.name }}
        </TButton>
      </div>
    </div>
  </div>
</template>
