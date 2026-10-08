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

import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { cn } from '@/methods/utils'

const props = withDefaults(
  defineProps<
    DialogRootProps & {
      title: string
      actionLabel?: string
      cancelLabel?: string
      actionDisabled?: boolean
      actionHref?: string
      loading?: boolean
      contentClass?: string
      disableContentPadding?: boolean
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
  'actionHref',
  'loading',
  'contentClass',
  'disableContentPadding',
)
const forwarded = useForwardPropsEmits(dialogRootProps, emit)
</script>

<template>
  <DialogRoot v-bind="forwarded">
    <DialogPortal>
      <DialogOverlay
        class="fixed inset-0 z-30 bg-surface-scrim fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in"
      />

      <DialogContent
        :class="
          cn(
            'fixed inset-0 z-30 m-auto flex h-max max-h-dvh w-max max-w-screen flex-col rounded-sm bg-surface-raised px-(--padding-x) py-section zoom-in-75 zoom-out-75 [--padding-x:--spacing(8)] fade-in fade-out data-[state=closed]:animate-out data-[state=open]:animate-in',
            $attrs.class,
          )
        "
      >
        <div class="mb-5 flex shrink-0 items-start justify-between gap-compact">
          <div class="flex flex-col">
            <DialogTitle class="font-medium text-content-emphasis">{{ title }}</DialogTitle>
            <DialogDescription v-if="$slots.description" class="text-sm">
              <slot name="description"></slot>
            </DialogDescription>
          </div>

          <DialogClose
            class="size-6 shrink-0 text-content-muted transition-colors hover:text-content-emphasis active:text-content-muted"
            aria-label="Close dialog"
          >
            <TIcon class="size-full" icon="close" />
          </DialogClose>
        </div>

        <div
          :class="
            cn(
              'min-h-0 grow overflow-y-auto',
              { '-mx-(--padding-x)': disableContentPadding },
              contentClass,
            )
          "
        >
          <slot></slot>
        </div>

        <div class="mt-section flex shrink-0 items-center justify-end gap-tight">
          <DialogClose as-child>
            <TButton variant="secondary">{{ cancelLabel }}</TButton>
          </DialogClose>

          <TButton
            v-if="actionLabel"
            :disabled="actionDisabled || loading"
            variant="highlighted"
            v-bind="actionHref ? { is: 'a', href: actionHref } : { is: 'button' }"
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
