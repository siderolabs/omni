<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { ref, useTemplateRef, watch } from 'vue'

import TInput from '@/components/TInput/TInput.vue'
import type { Label } from '@/methods/labels.ts'

import ItemLabel from './ItemLabel.vue'

const { match, completions } = defineProps<{
  match?: string
  completions: Label[]
}>()

const filterValue = defineModel<string>('filterValue', { required: true })
const filterLabels = defineModel<Label[]>('filterLabels', { required: true })

const showCompletions = ref(false)

const input = useTemplateRef('input')
const selectedSuggestion = ref(0)
const selectedLabel = ref<number>()

const autoComplete = (index: number) => {
  const label = completions[index]

  if (!label) {
    return
  }

  if (match && filterValue.value.endsWith(match)) {
    filterValue.value = filterValue.value.slice(0, -match.length)
  }

  addLabel(label)
}

const addLabel = (label: Label) => {
  if (filterLabels.value.some((l) => l.value === label.value && l.key === label.key)) {
    return
  }

  filterLabels.value = filterLabels.value.concat(label)
}

const removeLabel = (index: number) => {
  filterLabels.value = filterLabels.value.toSpliced(index, 1)
}

watch(filterValue, () => {
  selectedSuggestion.value = 0
  selectedLabel.value = undefined
})
</script>

<template>
  <div
    class="relative"
    @keydown.enter="autoComplete(selectedSuggestion)"
    @keydown.arrow-up.prevent="selectedSuggestion > 0 && selectedSuggestion--"
    @keydown.arrow-down="selectedSuggestion < completions.length - 1 && selectedSuggestion++"
    @keydown.backspace="
      () => {
        if (input?.getCaretPosition() !== 0) return

        if (selectedLabel !== undefined) {
          removeLabel(selectedLabel)

          selectedLabel = selectedLabel > 0 ? selectedLabel - 1 : undefined

          return
        }

        selectedLabel = filterLabels.length - 1
      }
    "
  >
    <TInput
      ref="input"
      v-model="filterValue"
      v-on-click-outside="() => (showCompletions = false)"
      class="h-full flex-1 flex-wrap text-xs"
      icon="search"
      clearable
      placeholder="Search ..."
      @clear="filterLabels = []"
      @click="showCompletions = true"
    >
      <template #labels>
        <div
          v-for="(label, index) in filterLabels"
          :key="label.key"
          class="-mx-1 -my-2 rounded-md border p-0.5 transition-all"
          :class="selectedLabel === index ? 'border-white' : 'border-transparent'"
        >
          <ItemLabel
            small
            :label="{ ...label, removable: true }"
            @remove-label="removeLabel(index)"
          />
        </div>
      </template>
    </TInput>

    <div
      v-if="completions.length > 0 && showCompletions"
      class="absolute top-full left-0 z-10 mt-1 flex min-w-full flex-col divide-y divide-naturals-n6 rounded border border-naturals-n4 bg-naturals-n2"
    >
      <div
        v-for="(suggestion, index) in completions"
        :key="index"
        class="flex cursor-pointer px-2 py-2 text-xs hover:bg-naturals-n4"
        :class="{ 'bg-naturals-n4': index === selectedSuggestion }"
        @click="autoComplete(index)"
      >
        <ItemLabel :label="{ ...suggestion, removable: false }" class="pointer-events-none" />
      </div>
    </div>
  </div>
</template>
