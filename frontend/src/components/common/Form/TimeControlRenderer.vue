<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsControl } from '@jsonforms/vue'
import { computed } from 'vue'

import { isChrome } from '@/methods'

import TIcon from '../Icon/TIcon.vue'
import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsControl(props)

const control = p.control

const dataTime = computed(() => (control.value.data ?? '').substr(0, 16))
</script>

<template>
  <ContentWrapper class="relative" :control="control">
    <input
      :id="control.id + '-input'"
      class="-my-1 bg-transparent text-xs text-naturals-n13 placeholder-naturals-n7 outline-hidden transition-colors focus:border-transparent focus:outline-hidden"
      type="time"
      :value="dataTime"
      :disabled="!control.enabled"
      @change="(event) => p.handleChange(control.path, (event.target as any)?.value)"
    />
    <div
      v-if="isChrome()"
      class="pointer-events-none absolute top-0 right-0 bottom-0 flex w-16 flex-1 items-center justify-center"
    >
      <TIcon icon="time" class="h-4 w-4" />
    </div>
  </ContentWrapper>
</template>

<style scoped>
@reference "../../../index.css";

input[type='time'] {
  @apply rounded border border-naturals-n7 px-2 py-1;
}

input[type='time']::-webkit-inner-spin-button {
  display: none;
}

input[type='time']::-webkit-calendar-picker-indicator {
  opacity: 0;
}
</style>
