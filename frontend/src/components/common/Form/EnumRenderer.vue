<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <content-wrapper :control="control">
    <t-select-list
      class="h-6"
      menu-align="right"
      :id="control.id + '-input'"
      :default-value="control.data ?? 'unset'"
      :disabled="!control.enabled"
      :values="values"
      @checked-value="
        (value) =>
          p.handleChange(control.path, control.options.find((item) => item.label === value)?.value)
      "
    />
  </content-wrapper>
</template>

<script setup lang="ts">
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsEnumControl } from '@jsonforms/vue'
import type { ControlElement } from '@jsonforms/core'

import TSelectList from '../SelectList/TSelectList.vue'
import { computed } from 'vue'
import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsEnumControl(props)

const control = p.control

const values = computed(() => {
  return control.value.options.map((item) => item.label)
})
</script>
