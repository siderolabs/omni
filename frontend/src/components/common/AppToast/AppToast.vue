<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import 'vue-sonner/style.css'

import { Toaster } from 'vue-sonner'

import TIcon from '@/components/common/Icon/TIcon.vue'
</script>

<template>
  <Toaster
    close-button
    :toast-options="{
      unstyled: true,
      classes: {
        toast: 'flex w-sm gap-2 rounded border border-naturals-n5 bg-naturals-n0 p-2',
        closeButton:
          'absolute top-2 right-2 rounded-full bg-naturals-n7 p-0.5 text-naturals-n14 hover:bg-naturals-n6 active:bg-naturals-n5',
        icon: 'size-5 shrink-0 self-center *:size-full',
        content: 'flex flex-col gap-1',
        title: 'text-sm text-naturals-n14',
        description: 'overflow-auto text-xs text-naturals-n11',

        error: 'border-l-4 border-l-red-r1 text-red-r1',
        success: 'border-l-4 border-l-green-g1 text-green-g1',
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
