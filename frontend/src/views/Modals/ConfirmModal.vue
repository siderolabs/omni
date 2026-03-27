<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  type AlertDialogEmits,
  AlertDialogOverlay,
  AlertDialogPortal,
  type AlertDialogProps,
  AlertDialogRoot,
  AlertDialogTitle,
  useForwardPropsEmits,
} from 'reka-ui'

import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'

const props = withDefaults(
  defineProps<
    AlertDialogProps & {
      title?: string
      actionLabel?: string
      loading?: boolean
    }
  >(),
  {
    title: 'Confirm Action',
    actionLabel: 'Confirm',
  },
)

const emit = defineEmits<AlertDialogEmits & { confirm: [] }>()

const alertDialogRootProps = reactiveOmit(props, 'title', 'actionLabel', 'loading')
const forwarded = useForwardPropsEmits(alertDialogRootProps, emit)
</script>

<template>
  <AlertDialogRoot v-bind="forwarded">
    <AlertDialogPortal>
      <AlertDialogOverlay
        class="fixed inset-0 z-30 bg-naturals-n0/90 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      />

      <AlertDialogContent
        class="fixed top-1/2 left-1/2 z-100 flex max-h-screen max-w-screen -translate-1/2 flex-col rounded-sm bg-naturals-n3 p-8 zoom-in-75 zoom-out-75 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      >
        <div class="mb-5 flex flex-col">
          <AlertDialogTitle class="font-medium text-naturals-n14">{{ title }}</AlertDialogTitle>
          <AlertDialogDescription v-if="$slots.description" class="text-sm">
            <slot name="description"></slot>
          </AlertDialogDescription>
        </div>

        <slot></slot>

        <div class="mt-8 flex items-center justify-end gap-2">
          <AlertDialogCancel as-child>
            <TButton variant="secondary">Cancel</TButton>
          </AlertDialogCancel>

          <TButton :disabled="loading" variant="highlighted" @click="$emit('confirm')">
            <TSpinner v-if="loading" class="size-5" />
            <template v-else>{{ actionLabel }}</template>
          </TButton>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
