<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsControl } from '@jsonforms/vue'

import TInput from '../TInput/TInput.vue'
import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsControl(props)

const control = p.control
</script>

<template>
  <ContentWrapper :control="control">
    <TInput
      :id="control.id + '-input'"
      type="text"
      :step="1"
      :disabled="!control.enabled"
      :model-value="control.data ?? ''"
      compact
      class="-my-2 min-w-56"
      @update:model-value="(value: string) => p.handleChange(control.path, value)"
    />
  </ContentWrapper>
</template>
