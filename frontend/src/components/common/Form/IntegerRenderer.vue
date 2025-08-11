<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import { Resolve } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsControl } from '@jsonforms/vue'
import { computed } from 'vue'

import TInput from '../TInput/TInput.vue'
import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsControl(props)

const control = p.control

const schema = computed(() => {
  return Resolve.schema(props.schema, control.value.uischema.scope, control.value.rootSchema)
})

const oldVal = control.value.data

if (schema.value.minimum) {
  control.value.data = Math.max(control.value.data, schema.value.minimum)
}

if (schema.value.maximum) {
  control.value.data = Math.min(control.value.data, schema.value.maximum)
}

if (oldVal !== control.value.data) {
  p.handleChange(control.value.path, control.value.data)
}
</script>

<template>
  <ContentWrapper :control="control">
    <TInput
      :id="control.id + '-input'"
      type="number"
      :step="1"
      :disabled="!control.enabled"
      :model-value="control.data ?? schema.minimum ?? 0"
      compact
      :min="schema.minimum"
      :max="schema.maximum"
      class="-my-2 min-w-56"
      @update:model-value="(value: number) => p.handleChange(control.path, value)"
    />
  </ContentWrapper>
</template>
