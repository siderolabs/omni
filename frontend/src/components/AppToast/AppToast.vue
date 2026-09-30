<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import 'vue-sonner/style.css'

import { Toaster } from 'vue-sonner'

import TIcon from '@/components/Icon/TIcon.vue'
</script>

<template>
  <Toaster
    close-button
    :offset="{
      // Custom offset to show toasts above the lower bars we sometimes have
      bottom: '4.5rem',
      right: '1rem',
    }"
    :toast-options="{
      unstyled: true,
      classes: {
        toast: 'flex w-sm gap-2 rounded border border-border-strong bg-surface-page p-2',
        closeButton:
          'absolute top-2 right-2 rounded-full bg-surface-inert p-0.5 text-content-emphasis hover:bg-surface-inert active:bg-surface-inert',
        icon: 'size-5 shrink-0 self-center *:size-full',
        content: 'flex flex-col gap-1',
        title: 'text-sm text-content-emphasis',
        description: 'overflow-auto text-xs whitespace-pre-wrap text-content-secondary',

        error: 'border-l-4 border-l-status-danger-default text-status-danger-text',
        success: 'border-l-4 border-l-status-success-default text-status-success-text',
        warning: 'border-l-4 border-l-status-warning-default text-status-warning-text',
      },
    }"
  >
    <template #success-icon>
      <TIcon icon="check-in-circle-classic" />
    </template>

    <template #error-icon>
      <TIcon icon="error" />
    </template>
  </Toaster>
</template>
