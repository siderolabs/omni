<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  type DialogRootEmits,
  type DialogRootProps,
  DialogTitle,
  useForwardPropsEmits,
} from 'reka-ui'

import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'

const props = withDefaults(
  defineProps<
    DialogRootProps & {
      title: string
      actionLabel: string
      cancelLabel?: string
      actionDisabled?: boolean
      loading?: boolean
    }
  >(),
  { cancelLabel: 'Cancel' },
)

const emit = defineEmits<DialogRootEmits & { confirm: [] }>()

const dialogRootProps = reactiveOmit(
  props,
  'title',
  'actionLabel',
  'cancelLabel',
  'actionDisabled',
  'loading',
)
const forwarded = useForwardPropsEmits(dialogRootProps, emit)
</script>

<template>
  <DialogRoot v-bind="forwarded">
    <DialogPortal>
      <DialogOverlay
        class="fixed inset-0 z-30 bg-naturals-n0/90 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      />

      <DialogContent
        class="fixed top-1/2 left-1/2 z-100 flex max-h-screen max-w-screen -translate-1/2 flex-col rounded-sm bg-naturals-n3 p-8 zoom-in-75 zoom-out-75 fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      >
        <div class="mb-5 flex items-start justify-between gap-4">
          <div class="flex flex-col">
            <DialogTitle class="font-medium text-naturals-n14">{{ title }}</DialogTitle>
            <DialogDescription v-if="$slots.description" class="text-sm">
              <slot name="description"></slot>
            </DialogDescription>
          </div>

          <DialogClose
            class="size-6 shrink-0 text-naturals-n10 transition-colors hover:text-naturals-n14 active:text-naturals-n9"
            aria-label="Close"
          >
            <TIcon class="size-full" icon="close" />
          </DialogClose>
        </div>

        <div class="overflow-y-auto">
          <slot></slot>
        </div>

        <div class="mt-8 flex items-center justify-end gap-2">
          <DialogClose as-child>
            <TButton variant="secondary">{{ cancelLabel }}</TButton>
          </DialogClose>

          <TButton
            :disabled="actionDisabled || loading"
            variant="highlighted"
            @click="$emit('confirm')"
          >
            <TSpinner v-if="loading" class="size-5" />
            <template v-else>{{ actionLabel }}</template>
          </TButton>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
