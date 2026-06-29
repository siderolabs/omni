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

import TButton from '@/components/Button/TButton.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { cn } from '@/methods/utils'

const props = withDefaults(
  // eslint-disable-next-line vue/define-props-destructuring
  defineProps<
    AlertDialogProps & {
      title?: string
      actionLabel?: string
      loading?: boolean
      disabled?: boolean
      contentClass?: string
    }
  >(),
  {
    title: 'Confirm Action',
    actionLabel: 'Confirm',
  },
)

const emit = defineEmits<AlertDialogEmits & { confirm: [] }>()

const alertDialogRootProps = reactiveOmit(
  props,
  'title',
  'actionLabel',
  'loading',
  'disabled',
  'contentClass',
)
const forwarded = useForwardPropsEmits(alertDialogRootProps, emit)
</script>

<template>
  <AlertDialogRoot v-bind="forwarded">
    <AlertDialogPortal>
      <AlertDialogOverlay
        class="fixed inset-0 z-30 bg-naturals-n0/90 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      />

      <AlertDialogContent
        class="fixed inset-0 z-30 m-auto flex h-max max-h-screen w-max max-w-screen flex-col rounded-sm bg-naturals-n3 p-8 zoom-in-75 zoom-out-75 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      >
        <div class="mb-5 flex shrink-0 flex-col">
          <AlertDialogTitle class="font-medium text-naturals-n14">{{ title }}</AlertDialogTitle>
          <AlertDialogDescription v-if="$slots.description" class="text-sm">
            <slot name="description"></slot>
          </AlertDialogDescription>
        </div>

        <div :class="cn('min-h-0 grow overflow-y-auto', contentClass)">
          <slot></slot>
        </div>

        <div class="mt-8 flex shrink-0 items-center justify-end gap-2">
          <AlertDialogCancel as-child>
            <TButton variant="secondary">Cancel</TButton>
          </AlertDialogCancel>

          <TButton :disabled="loading || disabled" variant="highlighted" @click="$emit('confirm')">
            <TSpinner v-if="loading" class="size-5" />
            <template v-else>{{ actionLabel }}</template>
          </TButton>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>
