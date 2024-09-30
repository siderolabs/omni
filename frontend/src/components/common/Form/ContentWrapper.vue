<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex justify-between py-3 px-3 items-center gap-2" v-if="control.label">
    <div class="text-naturals-N11 text-xs flex items-center gap-2">
      {{ control.label }}{{ description }}
      <tooltip :description="control.errors" v-if="control.errors">
        <t-icon icon="warning" class="text-yellow-Y1 w-4 h-4"/>
      </tooltip>
    </div>
    <slot/>
  </div>
  <div class="flex py-4 px-3 gap-3 items-center" v-else>
    <div class="flex-1">
      <slot/>
    </div>
    <tooltip :description="control.errors" v-if="control.errors">
      <t-icon icon="warning" class="text-yellow-Y1 w-4 h-4 -my-1.5"/>
    </tooltip>
  </div>
</template>

<script setup lang="ts">
import Tooltip from '../Tooltip/Tooltip.vue';
import TIcon from '../Icon/TIcon.vue';
import { computed } from 'vue';

const props = defineProps<{
  control: {
    label: string
    errors: string
    description?: string
  },
}>()

const description = computed(() => {
  return props.control.description ? ` (${props.control.description})` : "";
})
</script>
