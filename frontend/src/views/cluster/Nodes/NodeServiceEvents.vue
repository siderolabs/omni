<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ServiceEvent } from '@/api/talos/machine/machine.pb'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { relativeISO } from '@/methods/time'

defineProps<{
  events?: ServiceEvent[]
}>()

const eventStyle = (state: string) => {
  let color = 'bg-naturals-N7'
  let icon: IconType = 'question'

  switch (state) {
    case 'Running':
      color = 'bg-green-G2'
      icon = 'check'

      break
    case 'Starting':
    case 'Stopping':
    case 'Waiting':
      color = 'bg-yellow-Y2'
      icon = 'loading'

      break
    case 'Preparing':
      icon = 'time'

      break
    case 'Finished':
    case 'Failed':
      icon = 'error'
      color = 'bg-red-R1'

      break
  }

  return {
    color,
    icon,
  }
}
</script>

<template>
  <div class="-mb-2 mt-4 pl-1">
    <div class="flex h-full w-full flex-col gap-4 border-l-2 border-naturals-N4">
      <div v-for="event in events" :key="event.ts" class="grid grid-cols-6 gap-3">
        <div class="flex items-center gap-3">
          <div
            class="max-w-min rounded-full p-1"
            :class="eventStyle(event.state!).color"
            style="margin-left: -11px"
          >
            <TIcon :icon="eventStyle(event.state!).icon" class="h-3 w-3 text-white" />
          </div>
          <div class="font-bold">{{ event.state }}</div>
        </div>
        <div class="col-span-5">{{ relativeISO(event.ts!) }} {{ event.msg }}</div>
      </div>
    </div>
  </div>
</template>
