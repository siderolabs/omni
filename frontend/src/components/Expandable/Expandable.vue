<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onKeyStroke } from '@vueuse/core'

import IconButton from '@/components/Button/IconButton.vue'
import { cn } from '@/methods/utils'

defineOptions({ inheritAttrs: false })

defineProps<{ title?: string }>()

const expanded = defineModel<boolean>('expanded', { default: false })

onKeyStroke('Escape', (event) => {
  if (!expanded.value) return

  event.stopPropagation()
  expanded.value = false
})
</script>

<template>
  <Teleport to="body" :disabled="!expanded">
    <Transition
      enter-active-class="transition-opacity"
      leave-active-class="transition-opacity"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div v-if="expanded" class="fixed inset-0 z-40 bg-naturals-n0/90" @click="expanded = false" />
    </Transition>

    <div
      :data-state="expanded ? 'open' : 'closed'"
      :class="
        cn(
          expanded &&
            'fixed inset-0 z-50 flex flex-col gap-3 rounded-sm bg-naturals-n3 p-6 sm:inset-6',
          $attrs.class,
        )
      "
    >
      <div v-if="expanded" class="flex shrink-0 items-center justify-between gap-4">
        <span class="font-medium text-naturals-n14">{{ title }}</span>

        <IconButton
          icon="close"
          class="size-8 shrink-0"
          aria-label="Collapse"
          @click="expanded = false"
        />
      </div>

      <div :class="expanded ? 'min-h-0 grow' : 'contents'">
        <slot></slot>
      </div>
    </div>
  </Teleport>
</template>
