<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useElementSize } from '@vueuse/core'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import { useTemplateRef } from 'vue'
import type { ComponentProps } from 'vue-component-type-helpers'

import TButton from '@/components/common/Button/TButton.vue'

interface Props {
  variant?: ComponentProps<typeof TButton>['variant']
  size?: ComponentProps<typeof TButton>['size']
  actions: [string, string, ...string[]]
  disabled?: boolean
}

const { variant = 'primary', size = 'md' } = defineProps<Props>()

defineEmits<{
  click: [action: string]
}>()

const triggerRef = useTemplateRef('trigger')
const { width } = useElementSize(triggerRef)
</script>

<template>
  <DropdownMenuRoot>
    <div v-bind="$attrs" ref="trigger" class="inline-flex -space-x-px">
      <TButton
        :variant="variant"
        :size
        :disabled
        class="rounded-tr-none rounded-br-none"
        @click="() => $emit('click', actions[0])"
      >
        {{ actions[0] }}
      </TButton>

      <DropdownMenuTrigger aria-label="extra actions" as-child>
        <TButton
          :variant="variant"
          :size
          :disabled
          class="rounded-tl-none rounded-bl-none"
          :class="{
            'px-2': size === 'md',
            'px-1': size === 'sm',
          }"
          icon="arrow-down"
        />
      </DropdownMenuTrigger>
    </div>

    <DropdownMenuPortal>
      <DropdownMenuContent
        class="z-50 max-h-[min(--spacing(70),var(--reka-dropdown-menu-content-available-height))] min-w-(--reka-dropdown-menu-trigger-width) overflow-auto rounded-md bg-naturals-n3 p-1.5 text-xs slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        side="bottom"
        side-flip
        :side-offset="4"
        align="end"
        :style="{
          // Overriding to align to the whole button, not just the trigger
          '--reka-dropdown-menu-trigger-width': `${width}px`,
        }"
      >
        <DropdownMenuItem
          v-for="action in actions"
          :key="action"
          value="New Tab"
          class="cursor-pointer px-3 py-1.5 outline-none select-none hover:text-naturals-n13 focus:text-naturals-n13"
          @select="() => $emit('click', action)"
        >
          {{ action }}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
