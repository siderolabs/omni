<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  DropdownMenuContent,
  DropdownMenuItemIndicator,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import { computed } from 'vue'

import IconButton from '@/components/Button/IconButton.vue'
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import { type ThemePreference, useTheme } from '@/methods/theme'

const { preference } = useTheme()

const options: { value: ThemePreference; label: string; icon: IconType }[] = [
  { value: 'light', label: 'Light', icon: 'sun' },
  { value: 'dark', label: 'Dark', icon: 'moon-solid' },
  { value: 'dim', label: 'Dim', icon: 'moon' },
  { value: 'system', label: 'System', icon: 'computer-desktop' },
]

const current = computed(
  () =>
    options.find((option) => option.value === preference.value) ??
    options.find((option) => option.value === 'system')!,
)
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <IconButton :icon="current.icon" :aria-label="`Theme: ${current.label}`" />
    </DropdownMenuTrigger>

    <DropdownMenuPortal>
      <DropdownMenuContent
        class="z-50 min-w-36 origin-(--reka-dropdown-menu-content-transform-origin) rounded border border-border-default bg-surface-raised py-micro data-[side=bottom]:slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        align="end"
        side="bottom"
        :side-offset="10"
      >
        <DropdownMenuRadioGroup v-model="preference">
          <DropdownMenuRadioItem
            v-for="option in options"
            :key="option.value"
            :value="option.value"
            class="flex w-full cursor-pointer items-center gap-tight px-snug py-tight text-xs text-content-secondary transition-colors hover:text-content-default data-[state=checked]:text-content-emphasis"
          >
            <TIcon class="size-4" :icon="option.icon" />
            <span class="grow">{{ option.label }}</span>
            <DropdownMenuItemIndicator>
              <TIcon class="size-4" icon="check" />
            </DropdownMenuItemIndicator>
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
