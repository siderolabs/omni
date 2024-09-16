<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <json-forms
      v-if="schema && uiSchema"
        :data="modelValue"
        :renderers="Object.freeze(renderers)"
        :schema="schema"
        :uischema="uiSchema"
        @change="event => $emit('update:model-value', event.data)"
      />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRefs, watch } from "vue";
import { JsonForms } from "@jsonforms/vue";
import {
  UISchemaElement,
  Layout,
  Scoped,
  JsonSchema,
  isBooleanControl,
  isOneOfEnumControl,
  isEnumControl,
  isNumberControl,
  isIntegerControl,
  isDateTimeControl,
  isStringControl,
  isDateControl,
  isTimeControl,
  schemaTypeIs,
  rankWith
} from '@jsonforms/core';
import { vanillaRenderers } from "@jsonforms/vue-vanilla";
import BooleanRenderer from "./BooleanRenderer.vue";
import EnumOneOfRenderer from "./EnumOneOfRenderer.vue";
import EnumRenderer from "./EnumRenderer.vue";
import NumberRenderer from "./NumberRenderer.vue";
import IntegerRenderer from "./IntegerRenderer.vue";
import DateTimeControlRenderer from "./DateTimeControlRenderer.vue";
import StringRenderer from "./StringRenderer.vue";
import ArrayRenderer from "./ArrayRenderer.vue";
import DateControlRenderer from "./DateControlRenderer.vue";
import TimeControlRenderer from "./TimeControlRenderer.vue";

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
];

const props = defineProps<{
  jsonSchema: string
  modelValue: any
}>();

const { jsonSchema } = toRefs(props);
const schema = ref<JsonSchema>();
const err = ref<Error>();

defineEmits(['update:model-value'])

const renderSchema = () => {
  err.value = undefined;

  try {
    schema.value = JSON.parse(jsonSchema.value);
  } catch (e) {
    err.value = e;
  }
};

watch(jsonSchema, renderSchema);

renderSchema();

const uiSchema = computed(() => {
  if (!schema.value) {
    return;
  }

  const layout: Layout = {
    type: 'VerticalLayout',
    elements: [],
  };

  for (const key in schema.value.properties) {
    if (schema.value.properties[key].type === 'null') {
      continue;
    }

    layout.elements.push({
      type: 'Control',
      scope: `#/properties/${key}`
    } as Scoped & UISchemaElement);
  }

  return layout;
})
</script>

<style scoped>
.vertical-layout {
  @apply flex flex-col divide-y divide-naturals-N4;
}
</style>

<style>
.group {
  @apply m-2 border border-naturals-N6 rounded;
}

.group > .group-item:not(:first-of-type) {
  @apply border-t border-naturals-N4;
}

.group-label {
  @apply px-1 ml-1 mt-3 text-naturals-N13 -mb-1.5;
}
</style>
