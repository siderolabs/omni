<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="p-2" v-if="control.visible">
    <div class="rounded border border-naturals-N5 flex-1 relative pt-3 pt-4">
      <div class="absolute -top-2 bg-naturals-N2 px-1 left-1 text-naturals-N13">
        {{ control.label }}
      </div>
      <t-button
        type="subtle-xs"
        class="text-xs mx-4 mb-3"
        icon="plus"
        :disabled="!control.enabled || maxItemsReached"
        @click="addButtonClicked"
      >
        Add
      </t-button>
      <div
        class="flex flex-col divide-y divide-naturals-N4 border-t border-naturals-N4"
        v-if="control.data?.length"
      >
        <div
          v-for="(_, index) in control.data"
          :key="`${control.path}-${index}`"
          class="py-1 items-center flex px-3"
        >
          <icon-button
            v-if="p.moveUp"
            icon="arrow-up"
            :disabled="index === 0"
            @click="() => moveUpClicked(index)"
          />
          <icon-button
            v-if="p.moveDown"
            icon="arrow-down"
            :disabled="index === control.data.length - 1"
            @click="() => moveDownClicked(index)"
          />
          <dispatch-renderer
            class="flex-1"
            :schema="control.schema"
            :uischema="childUiSchema"
            :path="composePaths(control.path, `${index}`)"
            :enabled="control.enabled"
            :renderers="control.renderers"
            :cells="control.cells"
          />
          <icon-button
            v-if="p.removeItems"
            icon="delete"
            @click="() => deleteButtonClicked(index)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { RendererProps } from '@jsonforms/vue'
import { useJsonFormsArrayControl, DispatchRenderer } from '@jsonforms/vue'
import type { ControlElement } from '@jsonforms/core'
import { composePaths, findUISchema, Resolve, createDefaultValue } from '@jsonforms/core'
import { computed } from 'vue'
import TButton from '../Button/TButton.vue'
import IconButton from '../Button/IconButton.vue'

const props = defineProps<RendererProps<ControlElement>>()

const p = useJsonFormsArrayControl(props)

const control = p.control

const childUiSchema = computed(() =>
  findUISchema(
    control.value.uischemas,
    control.value.schema,
    control.value.uischema.scope,
    control.value.path,
    undefined,
    control.value.uischema,
    control.value.rootSchema,
  ),
)

const arraySchema = computed(() => {
  return Resolve.schema(props.schema, control.value.uischema.scope, control.value.rootSchema)
})

const minItemsReached = computed(() => {
  return (
    arraySchema.value !== undefined &&
    arraySchema.value.minItems !== undefined &&
    control.value.data !== undefined &&
    control.value.data.length <= arraySchema.value.minItems
  )
})

const maxItemsReached = computed(() => {
  return (
    arraySchema.value !== undefined &&
    arraySchema.value.maxItems !== undefined &&
    control.value.data !== undefined &&
    control.value.data.length >= arraySchema.value.maxItems
  )
})

const addButtonClicked = () => {
  p.addItem(
    control.value.path,
    createDefaultValue(control.value.schema, control.value.rootSchema),
  )()
}

const deleteButtonClicked = (index: number) => {
  if (!p.removeItems || minItemsReached.value) {
    return
  }

  p.removeItems(control.value.path, [index])()
}

const moveUpClicked = (index: number) => {
  if (!p.moveUp) {
    return
  }

  p.moveUp(control.value.path, index)()
}

const moveDownClicked = (index: number) => {
  if (!p.moveDown) {
    return
  }

  p.moveDown(control.value.path, index)()
}
</script>
