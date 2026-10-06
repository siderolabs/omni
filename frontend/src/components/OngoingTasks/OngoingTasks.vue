<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { ref } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import { useOngoingTasks } from '@/methods/ongoingTasks'
import { formatISO } from '@/methods/time'

const { data } = useOngoingTasks()

const dropdownOpen = ref(false)
</script>

<template>
  <PopoverRoot v-model:open="dropdownOpen">
    <PopoverTrigger
      class="flex items-center gap-1 text-content-secondary transition-colors hover:text-content-emphasis"
    >
      <TIcon
        :icon="data.length > 0 ? 'action-horizontal-animated' : 'action-horizontal'"
        class="size-4"
      />
      <span class="text-xs font-normal whitespace-nowrap select-none">
        <span class="contents sm:hidden">Tasks</span>
        <span class="hidden sm:contents">Ongoing Tasks</span>
      </span>
      <TIcon
        class="flex h-4 w-4 items-center justify-center transition-transform duration-300"
        :class="{ '-rotate-180': dropdownOpen }"
        icon="dropdown"
      />
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="z-30 max-h-(--reka-popover-content-available-height) max-w-[min(--spacing(80),var(--reka-popover-content-available-width))] min-w-(--reka-popover-trigger-width) origin-(--reka-popover-content-transform-origin) overflow-auto rounded border border-border-default bg-surface-card slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        :side-offset="10"
        align="end"
        side="bottom"
      >
        <div
          v-for="{ item, desc } in data"
          :key="item.metadata.id"
          class="flex flex-col gap-2 border-border-default p-6 not-last:border-b"
        >
          <div class="flex items-center justify-between gap-4">
            <h3 class="truncate text-xs text-content-default">
              {{ item.spec.title }}
            </h3>

            <span
              v-if="item.metadata.created"
              class="shrink-0 text-right text-xs whitespace-nowrap text-content-muted"
            >
              {{ formatISO(item.metadata.created, 'HH:mm:ss') }}
            </span>
          </div>

          <div class="text-xs text-content-muted">
            <div v-if="desc.fromVersion && desc.toVersion" class="flex items-center gap-2 text-xs">
              <span class="whitespace-nowrap">{{ desc.action }}</span>
              <div class="flex-1" />
              <span
                class="truncate rounded bg-surface-hover px-2 text-xs font-bold text-content-default"
              >
                {{ desc.fromVersion }}
              </span>
              <span>⇾</span>
              <span
                class="truncate rounded bg-surface-hover px-2 text-xs font-bold text-content-default"
              >
                {{ desc.toVersion }}
              </span>
            </div>

            <div v-else-if="desc.revertingTo" class="flex items-center gap-2 text-xs">
              <span class="whitespace-nowrap">Reverting back to</span>
              <div class="flex-1" />
              <span
                class="rounded bg-surface-hover px-2 text-xs font-bold whitespace-nowrap text-content-default"
              >
                {{ desc.revertingTo }}
              </span>
            </div>

            <span v-else-if="desc.component" class="whitespace-nowrap">
              {{ desc.action }} · {{ desc.component }}
            </span>

            <span v-else class="whitespace-nowrap">{{ desc.action }}</span>
          </div>
        </div>

        <div v-if="!data.length" class="flex w-32 items-center justify-center gap-2 p-4 text-xs">
          <TIcon icon="check" class="h-4 w-4" />
          No tasks
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
