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
    class="rounded-md border border-l-4 border-naturals-n6 bg-naturals-n0 p-4"
    :class="{
      'border-l-red-r2': type === 'error',
      'border-l-blue-400': type === 'info',
      'border-l-green-g1': type === 'success',
      'border-l-yellow-y1': type === 'warn',
    }"
  >
    <div class="flex items-center">
      <div
        class="flex items-center justify-center"
        :class="{
          'text-red-400': type === 'error',
          'text-blue-400': type === 'info',
          'text-green-g1': type === 'success',
          'text-yellow-y1': type === 'warn',
        }"
      >
        <TIcon :icon="icons[type]" class="size-5" />
      </div>
      <div class="ml-3 flex flex-col gap-2">
        <h3
          class="text-sm font-medium"
          :class="{
            'text-red-400': type === 'error',
            'text-blue-400': type === 'info',
            'text-green-g1': type === 'success',
            'text-yellow-y1': type === 'warn',
          }"
        >
          {{ title }}
        </h3>
        <div v-if="$slots.default" class="text-sm font-normal">
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
