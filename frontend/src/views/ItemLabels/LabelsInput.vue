<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { computed, ref, useTemplateRef, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type { LabelsCompletionSpec } from '@/api/omni/specs/virtual.pb'
import TInput from '@/components/TInput/TInput.vue'
import { getLabelFromID as createLabel } from '@/methods/labels'
import { useResourceGet } from '@/methods/useResourceGet'

import ItemLabel from './ItemLabel.vue'

type Label = {
  id: string
  value: string
  key: string
  labelClass?: string
}

const { completionsResource } = defineProps<{
  completionsResource: GetRequest
}>()

const filterValue = defineModel<string>('filterValue', { required: true })
const filterLabels = defineModel<Label[]>('filterLabels', { required: true })

const showCompletions = ref(false)

const input = useTemplateRef('input')
const selectedSuggestion = ref(0)
const selectedLabel = ref<number>()

const { data: completion, loadData } = useResourceGet<LabelsCompletionSpec>(() => ({
  skip: true,
  runtime: Runtime.Omni,
  resource: completionsResource,
}))

const labelsCompletions = computed(() => {
  const completionEntries = Object.entries(completion.value?.spec.items ?? {})

  return completionEntries.flatMap(([key, { items = [] }]) => {
    const uniqItems = [...new Set(['', ...items])]

    return uniqItems.map((value) => ({ key, value }))
  })
})

// we always do completion for the last space separated word
const matchValue = computed(() => filterValue.value.split(' ').at(-1))

const matchedLabelsCompletion = computed(() => {
  if (!matchValue.value) return []

  const [key, value] = matchValue.value.split(':') as [string, string | undefined]

  return labelsCompletions.value
    .filter((item) =>
      value === undefined
        ? item.key.includes(key) || item.value.includes(key)
        : item.key.includes(key) && item.value.includes(value),
    )
    .map((item) => {
      const label = createLabel(item.key, item.value)

      label.id = item.value === '' ? `has label: ${label.id}` : label.id

      return label
    })
})

const autoComplete = (index: number) => {
  const label = matchedLabelsCompletion.value[index]

  if (!label) {
    return
  }

  if (matchValue.value && filterValue.value.endsWith(matchValue.value)) {
    filterValue.value = filterValue.value.slice(0, -matchValue.value.length)
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

let abortController: AbortController | null

watch(filterValue, async (val, old, onCleanup) => {
  onCleanup(() => {
    selectedSuggestion.value = 0
    selectedLabel.value = undefined
    abortController?.abort({ reason: 'input changed' })
  })

  if (old === '' || abortController) {
    abortController = new AbortController()

    await loadData(abortController)

    abortController = null
  }
})
</script>

<template>
  <div
    class="relative"
    @keydown.enter="() => autoComplete(selectedSuggestion)"
    @keydown.arrow-up.prevent="
      () => {
        if (selectedSuggestion > 0) {
          selectedSuggestion--
        }
      }
    "
    @keydown.backspace="
      () => {
        if (input?.getCaretPosition() !== 0) {
          return
        }

        if (selectedLabel !== undefined) {
          removeLabel(selectedLabel)

          if (selectedLabel > 0) {
            selectedLabel--
          } else {
            selectedLabel = undefined
          }

          return
        }

        selectedLabel = filterLabels.length - 1
      }
    "
    @keydown.arrow-down="
      () => {
        if (selectedSuggestion < matchedLabelsCompletion.length - 1) {
          selectedSuggestion++
        }
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
      v-if="matchedLabelsCompletion.length > 0 && showCompletions"
      class="absolute top-full left-0 z-10 mt-1 flex min-w-full flex-col divide-y divide-naturals-n6 rounded border border-naturals-n4 bg-naturals-n2"
    >
      <div
        v-for="(suggestion, index) in matchedLabelsCompletion"
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
