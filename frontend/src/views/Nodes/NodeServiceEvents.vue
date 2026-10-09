<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ServiceEvent } from '@/api/talos/machine/machine.pb'
import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import { relativeISO } from '@/methods/time'

defineProps<{
  events?: ServiceEvent[]
}>()

const eventStyle = (state: string) => {
  let color = 'bg-status-info-fill text-status-info-on-fill'
  let icon: IconType = 'question-mark-circle'

  switch (state) {
    case 'Running':
      color = 'bg-status-success-fill text-status-success-on-fill'
      icon = 'check'

      break
    case 'Starting':
    case 'Stopping':
    case 'Waiting':
      color = 'bg-status-warning-fill text-status-warning-on-fill'
      icon = 'loading'

      break
    case 'Preparing':
      icon = 'time'

      break
    case 'Finished':
      icon = 'stop'

      break
    case 'Failed':
    case 'Corrupted':
      icon = 'x-circle'
      color = 'bg-status-danger-fill text-status-danger-on-fill'

      break
  }

  return {
    color,
    icon,
  }
}
</script>

<template>
  <div class="pl-micro">
    <div class="flex h-full w-full flex-col gap-compact border-l-2 border-border-default">
      <div v-for="event in events" :key="event.ts" class="grid grid-cols-6 gap-snug">
        <div class="flex items-center gap-snug">
          <div
            class="max-w-min rounded-full p-micro"
            :class="eventStyle(event.state!).color"
            style="margin-left: -11px"
          >
            <TIcon :icon="eventStyle(event.state!).icon" class="h-3 w-3" />
          </div>
          <div class="font-bold">{{ event.state }}</div>
        </div>
        <div class="col-span-5">{{ relativeISO(event.ts!) }} {{ event.msg }}</div>
      </div>
    </div>
  </div>
</template>
