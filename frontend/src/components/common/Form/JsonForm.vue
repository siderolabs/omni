<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { JsonSchema, Layout, Scoped, UISchemaElement } from '@jsonforms/core'
import {
  isBooleanControl,
  isDateControl,
  isDateTimeControl,
  isEnumControl,
  isIntegerControl,
  isNumberControl,
  isOneOfEnumControl,
  isStringControl,
  isTimeControl,
  rankWith,
  schemaTypeIs,
} from '@jsonforms/core'
import { JsonForms } from '@jsonforms/vue'
import { vanillaRenderers } from '@jsonforms/vue-vanilla'
import { type ErrorObject } from 'ajv'
import { dump } from 'js-yaml'
import { computed, ref, toRefs, watch } from 'vue'

import { ManagementService } from '@/api/omni/management/management.pb'

import ArrayRenderer from './ArrayRenderer.vue'
import BooleanRenderer from './BooleanRenderer.vue'
import DateControlRenderer from './DateControlRenderer.vue'
import DateTimeControlRenderer from './DateTimeControlRenderer.vue'
import EnumOneOfRenderer from './EnumOneOfRenderer.vue'
import EnumRenderer from './EnumRenderer.vue'
import IntegerRenderer from './IntegerRenderer.vue'
import NumberRenderer from './NumberRenderer.vue'
import StringRenderer from './StringRenderer.vue'
import TimeControlRenderer from './TimeControlRenderer.vue'

const errors = ref<ErrorObject[]>([])

const emit = defineEmits(['update:model-value'])

const onChange = async (event: { data: unknown; errors: unknown }) => {
  emit('update:model-value', event.data)
}

const renderers = [
  {
    renderer: BooleanRenderer,
    tester: rankWith(25, isBooleanControl),
  },
  {
    renderer: EnumOneOfRenderer,
    tester: rankWith(25, isOneOfEnumControl),
  },
  {
    renderer: EnumRenderer,
    tester: rankWith(25, isEnumControl),
  },
  {
    renderer: NumberRenderer,
    tester: rankWith(25, isNumberControl),
  },
  {
    renderer: IntegerRenderer,
    tester: rankWith(25, isIntegerControl),
  },
  {
    renderer: DateTimeControlRenderer,
    tester: rankWith(25, isDateTimeControl),
  },
  {
    renderer: StringRenderer,
    tester: rankWith(25, isStringControl),
  },
  {
    renderer: ArrayRenderer,
    tester: rankWith(25, schemaTypeIs('array')),
  },
  {
    renderer: DateControlRenderer,
    tester: rankWith(50, isDateControl),
  },
  {
    renderer: TimeControlRenderer,
    tester: rankWith(50, isTimeControl),
  },
  ...vanillaRenderers,
]

const props = defineProps<{
  jsonSchema: string
  modelValue: unknown
}>()

const { jsonSchema, modelValue } = toRefs(props)
const schema = ref<JsonSchema>()
const err = ref<Error>()

const renderSchema = () => {
  err.value = undefined

  try {
    schema.value = JSON.parse(jsonSchema.value)
  } catch (e) {
    err.value = e
  }
}

watch(modelValue, (val) => {
  updateErrors(val)
})

const updateErrors = async (data: unknown) => {
  if (!jsonSchema.value) {
    return
  }

  const response = await ManagementService.ValidateJSONSchema({
    schema: jsonSchema.value,
    data: dump(data),
  })

  errors.value =
    response.errors?.map<ErrorObject>((item) => ({
      // TODO: Required props for ErrorObject, but possible that this error feature is not working currently
      keyword: '',
      instancePath: '',
      params: {},
      schemaPath: item.schema_path!,
      dataPath: item.data_path!,
      message: item.cause,
    })) ?? []
}

watch(jsonSchema, renderSchema)

renderSchema()

updateErrors(modelValue.value)

const uiSchema = computed(() => {
  if (!schema.value) {
    return
  }

  const layout: Layout = {
    type: 'VerticalLayout',
    elements: [],
  }

  for (const key in schema.value.properties) {
    if (schema.value.properties[key].type === 'null') {
      continue
    }

    layout.elements.push({
      type: 'Control',
      scope: `#/properties/${key}`,
    } as Scoped & UISchemaElement)
  }

  return layout
})
</script>

<template>
  <div>
    <JsonForms
      v-if="schema && uiSchema"
      :data="modelValue"
      :renderers="Object.freeze(renderers)"
      :schema="schema"
      :uischema="uiSchema"
      :additional-errors="errors"
      validation-mode="NoValidation"
      @change="onChange"
    />
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.vertical-layout {
  @apply flex flex-col divide-y divide-naturals-n4;
}
</style>

<style>
@reference "../../../index.css";
.group {
  @apply m-2 rounded border border-naturals-n6;
}

.group > .group-item:not(:first-of-type) {
  @apply border-t border-naturals-n4;
}

.group-label {
  @apply mt-3 -mb-1.5 ml-1 px-1 text-naturals-n13;
}
</style>
