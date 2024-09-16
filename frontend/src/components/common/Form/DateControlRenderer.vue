<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <content-wrapper class="relative" :control="control">
    <input
      class="bg-transparent outline-none text-naturals-N13 focus:outline-none focus:border-transparent text-xs transition-colors placeholder-naturals-N7 -my-1"
      :id="control.id + '-input'"
      type="date"
      :value="dataTime"
      :disabled="!control.enabled"
      @change="(event) => p.handleChange(control.path, (event.target as any)?.value)"
    />
    <div class="absolute flex flex-1 top-0 right-0 bottom-0 w-16 items-center justify-center pointer-events-none" v-if="isChrome()">
      <t-icon icon="calendar" class="w-4 h-4"/>
    </div>
  </content-wrapper>
</template>

<script setup lang="ts">
import {
  RendererProps,
  useJsonFormsControl,
} from "@jsonforms/vue";
import {
  ControlElement,
} from "@jsonforms/core";
import ContentWrapper from "./ContentWrapper.vue";
import { computed } from "vue";
import TIcon from "../Icon/TIcon.vue";
import { isChrome } from "@/methods";

const props = defineProps<RendererProps<ControlElement>>();

const p = useJsonFormsControl(props);

const control = p.control

const dataTime = computed(() => (control.value.data ?? '').substr(0, 16))
</script>

<style scoped>
input[type="date"] {
  @apply border border-naturals-N7 px-2 py-1 rounded;
}

input[type="date"]::-webkit-inner-spin-button {
  display: none;
  -webkit-appearance: none;
}

input[type="date"]::-webkit-calendar-picker-indicator {
  opacity: 0;
}
</style>
