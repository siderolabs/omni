<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsEnumControl } from '@jsonforms/vue'
import { computed } from 'vue'

import TSelectList from '../SelectList/TSelectList.vue'
import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsEnumControl(props)

const control = p.control

const values = computed(() => {
  return control.value.options.map((item) => item.label)
})
</script>

<template>
  <ContentWrapper :control="control">
    <TSelectList
      :id="control.id + '-input'"
      class="h-6"
      menu-align="right"
      :default-value="control.data ?? 'unset'"
      :disabled="!control.enabled"
      :values="values"
      @checked-value="
        (value) =>
          p.handleChange(control.path, control.options.find((item) => item.label === value)?.value)
      "
    />
  </ContentWrapper>
</template>
