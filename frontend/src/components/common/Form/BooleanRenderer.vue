<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsControl } from '@jsonforms/vue'

import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'

import ContentWrapper from './ContentWrapper.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsControl(props)

const control = p.control
</script>

<template>
  <ContentWrapper :control="control">
    <TCheckbox
      :id="control.id + '-input'"
      :checked="control.data"
      :disabled="!control.enabled"
      @click="() => p.handleChange(control.path, !control.data)"
    />
  </ContentWrapper>
</template>
