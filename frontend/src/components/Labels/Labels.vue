<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface LabelSelectItem {
  value: string
  canRemove?: boolean
  labelClass?: string
}
</script>

<script setup lang="ts">
import { ref } from 'vue'

import TButton from '@/components/Button/TButton.vue'
import TInput from '@/components/TInput/TInput.vue'
import ItemLabel from '@/views/ItemLabels/ItemLabel.vue'

const modelValue = defineModel<Record<string, LabelSelectItem>>()

const addingLabel = ref(false)
const currentLabel = ref('')

const editLabels = () => {
  addingLabel.value = true
}

const addLabel = async () => {
  addingLabel.value = false

  if (!currentLabel.value.trim()) {
    return
  }

  const [key, value] = currentLabel.value.split(':')

  currentLabel.value = ''

  modelValue.value = {
    ...modelValue.value,
    [key.trim()]: {
      value: value?.trim() ?? '',
      canRemove: true,
    },
  }
}

const removeLabel = async (key: string) => {
  addingLabel.value = false

  currentLabel.value = ''

  const copied = { ...modelValue.value }

  delete copied[key]

  modelValue.value = copied
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-1.5 text-xs">
    <ItemLabel
      v-for="(label, key) in modelValue"
      :key="key"
      :label="{
        key,
        id: key,
        value: label.value,
        labelClass: label.labelClass,
        removable: label.canRemove,
      }"
      :remove-label="removeLabel"
    />
    <TInput
      v-if="addingLabel"
      v-model="currentLabel"
      compact
      :focus="addingLabel"
      class="h-6 w-24"
      @keydown.enter="addLabel"
      @click.stop
      @blur="addLabel"
    />
    <TButton icon="tag" size="sm" @click.stop="editLabels">new label</TButton>
  </div>
</template>
