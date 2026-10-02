<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Toaster } from 'vue-sonner'

import TIcon from '@/components/Icon/TIcon.vue'
import { colorScheme, useTheme } from '@/methods/theme'

const { theme } = useTheme()
</script>

<template>
  <Toaster
    :theme="colorScheme(theme)"
    close-button
    :offset="{
      // Custom offset to show toasts above the lower bars we sometimes have
      bottom: '4.5rem',
      right: '1rem',
    }"
    :toast-options="{
      unstyled: true,
      classes: {
        toast:
          'flex w-sm gap-tight rounded border border-border-strong bg-surface-raised p-tight shadow-dropdown',
        closeButton:
          'absolute top-2 right-2 rounded-full bg-surface-hover p-micro text-content-emphasis hover:bg-surface-inert',
        icon: 'size-5 shrink-0 self-center *:size-full',
        content: 'flex flex-col gap-micro',
        title: 'text-sm text-content-emphasis',
        description: 'overflow-auto text-xs whitespace-pre-wrap text-content-secondary',

        error: 'border-l-4 border-l-status-danger-fill',
        success: 'border-l-4 border-l-status-success-fill',
        warning: 'border-l-4 border-l-status-warning-fill',
      },
    }"
  >
    <template #success-icon>
      <TIcon icon="check-circle" class="size-4 text-status-success-default" />
    </template>

    <template #error-icon>
      <TIcon icon="x-circle" class="size-4 text-status-danger-default" />
    </template>

    <template #warning-icon>
      <TIcon icon="exclamation-triangle" class="size-4 text-status-warning-default" />
    </template>
  </Toaster>
</template>

<!-- eslint-disable-next-line vue/enforce-style-attribute -->
<style>
/*
 * Some vue-sonner classes don't obey "unstyled: true"
 * Importing here inside the components layer so that
 * tailwind overrides will be specific enough to replace them.
 */
@import 'vue-sonner/style.css' layer(components);
</style>
