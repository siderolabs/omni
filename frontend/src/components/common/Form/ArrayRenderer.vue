<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ControlElement } from '@jsonforms/core'
import { composePaths, createDefaultValue, findUISchema, Resolve } from '@jsonforms/core'
import type { RendererProps } from '@jsonforms/vue'
import { DispatchRenderer, useJsonFormsArrayControl } from '@jsonforms/vue'
import { computed } from 'vue'

import IconButton from '../Button/IconButton.vue'
import TButton from '../Button/TButton.vue'

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

<template>
  <div v-if="control.visible" class="p-2">
    <div class="relative flex-1 rounded border border-naturals-N5 pt-3 pt-4">
      <div class="absolute -top-2 left-1 bg-naturals-N2 px-1 text-naturals-N13">
        {{ control.label }}
      </div>
      <TButton
        type="subtle-xs"
        class="mx-4 mb-3 text-xs"
        icon="plus"
        :disabled="!control.enabled || maxItemsReached"
        @click="addButtonClicked"
      >
        Add
      </TButton>
      <div
        v-if="control.data?.length"
        class="flex flex-col divide-y divide-naturals-N4 border-t border-naturals-N4"
      >
        <div
          v-for="(_, index) in control.data"
          :key="`${control.path}-${index}`"
          class="flex items-center px-3 py-1"
        >
          <IconButton
            v-if="p.moveUp"
            icon="arrow-up"
            :disabled="index === 0"
            @click="() => moveUpClicked(index)"
          />
          <IconButton
            v-if="p.moveDown"
            icon="arrow-down"
            :disabled="index === control.data.length - 1"
            @click="() => moveDownClicked(index)"
          />
          <DispatchRenderer
            class="flex-1"
            :schema="control.schema"
            :uischema="childUiSchema"
            :path="composePaths(control.path, `${index}`)"
            :enabled="control.enabled"
            :renderers="control.renderers"
            :cells="control.cells"
          />
          <IconButton
            v-if="p.removeItems"
            icon="delete"
            @click="() => deleteButtonClicked(index)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
