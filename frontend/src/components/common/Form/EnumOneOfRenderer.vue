<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <content-wrapper :control="control">
    <t-select-list
      class="h-6"
      :id="control.id + '-input'"
      :default-value="control.data"
      :disabled="!control.enabled"
      :values="values"
      @checked-value="(value) => p.handleChange(control.path, control.options.find(item => item.label === value)?.value)"/>
  </content-wrapper>
</template>

<script setup lang="ts">
import {
  useJsonFormsOneOfEnumControl,
  RendererProps,
} from "@jsonforms/vue";
import {
  ControlElement
} from "@jsonforms/core";

import TSelectList from '../SelectList/TSelectList.vue';
import { computed } from "vue";
import ContentWrapper from "./ContentWrapper.vue";

const props = defineProps<RendererProps<ControlElement>>();

const p = useJsonFormsOneOfEnumControl(props);

const control = p.control

const values = computed(() => {
  return control.value.options.map(item => item.label);
});
</script>
