<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="pl-1 mt-4 -mb-2">
    <div class="w-full h-full border-l-2 border-naturals-N4 flex flex-col gap-4">
      <div v-for="event in events" :key="event.ts" class="grid grid-cols-6 gap-3">
        <div class="flex items-center gap-3">
          <div class="rounded-full p-1 max-w-min " :class="eventStyle(event.state!).color" style="margin-left: -11px">
            <t-icon :icon="eventStyle(event.state!).icon" class="w-3 h-3 text-white"/>
          </div>
          <div class="font-bold">{{ event.state }}</div>
        </div>
        <div class="col-span-5">{{ relativeISO(event.ts!) }} {{ event.msg }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ServiceEvent } from "@/api/talos/machine/machine.pb";

import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { relativeISO } from "@/methods/time";

defineProps<{
  events?: ServiceEvent[],
}>();

const eventStyle = (state: string) => {
  let color = "bg-naturals-N7";
  let icon: IconType = "question";

  switch (state) {
  case "Running":
    color = "bg-green-G2";
    icon = "check"

    break;
  case "Starting":
  case "Stopping":
  case "Waiting":
    color = "bg-yellow-Y2";
    icon = "loading"

    break;
  case "Preparing":
    icon = "time"

    break;
  case "Finished":
  case "Failed":
    icon = "error";
    color = "bg-red-R1";

    break;
  }

  return {
    color,
    icon
  }
}
</script>
