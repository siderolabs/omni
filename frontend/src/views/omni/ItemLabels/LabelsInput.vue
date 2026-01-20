<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { ref, toRefs, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { withAbortController, withRuntime } from '@/api/options'
import TInput from '@/components/common/TInput/TInput.vue'
import { getLabelFromID as createLabel } from '@/methods/labels'

import ItemLabel from '../ItemLabels/ItemLabel.vue'

type Label = {
  id: string
  value: string
  key: string
  color: string
}

const props = defineProps<{
  completionsResource: {
    id: string
    namespace: string
    type: string
  }
  filterValue: string
  filterLabels: Label[]
  placeholder?: string
}>()

const showCompletions = ref(false)

const emit = defineEmits(['update:filter-value', 'update:filter-labels'])

const { filterValue, filterLabels } = toRefs(props)

const input = ref<{ getCaretPosition: () => number | void }>()
const selectedSuggestion = ref(0)
const selectedLabel = ref<number>()

const matchedLabelsCompletion = ref<Label[]>([])

let labelsCompletions: { key: string; value: string }[] = []
let matchValue = ''

const autoComplete = (index: number) => {
  const label = matchedLabelsCompletion.value[index]

  if (!label) {
    return
  }

  emit('update:filter-value', filterValue.value.replace(new RegExp(`${matchValue}$`), ''))

  addLabel(label)
}

const addLabel = (label: Label) => {
  if (filterLabels.value.find((l) => l.value === label.value && l.key === label.key)) {
    return
  }

  emit('update:filter-labels', filterLabels.value.concat([label]))
}

const removeLabel = (index: number) => {
  const copyArray = [...filterLabels.value]

  copyArray.splice(index, 1)

  emit('update:filter-labels', copyArray)
}

let abortController: AbortController | null

watch(filterValue, async (val: string, old: string) => {
  selectedSuggestion.value = 0
  selectedLabel.value = undefined

  if (abortController) {
    abortController.abort({ reason: 'input changed' })
  }

  if (old === '' || abortController) {
    abortController = new AbortController()

    try {
      const completion: Resource<{
        items: Record<
          string,
          {
            items: string[]
          }
        >
      }> = await ResourceService.Get(
        props.completionsResource,
        withRuntime(Runtime.Omni),
        withAbortController(abortController),
      )

      abortController = null

      labelsCompletions = []

      const addLabel = (l: { key: string; value: string }) => {
        if (labelsCompletions.find((item) => item.key === l.key && item.value === l.value)) {
          return
        }

        labelsCompletions.push(l)
      }

      for (const key in completion.spec.items) {
        let hasEmptyValue = false

        for (const value of completion.spec.items[key].items!) {
          addLabel({
            key: key,
            value: value,
          })

          if (!value) {
            hasEmptyValue = true
          }
        }

        if (!hasEmptyValue) {
          addLabel({
            key: key,
            value: '',
          })
        }
      }
    } catch (e) {
      if (e.reason !== 'input changed') {
        throw e
      }
    }
  }

  // we always do completion for the last space separated word
  const parts = val.split(' ')

  matchValue = parts[parts.length - 1]

  const keyAndValue = matchValue.split(':')

  if (matchValue === '') {
    matchedLabelsCompletion.value = []

    return
  }

  const matcher = (item: { key: string; value: string }) => {
    const key = keyAndValue[0]
    const value = keyAndValue[1]

    if (value === undefined) {
      return item.key.includes(key) || item.value.includes(key)
    }

    return item.key.includes(key) && item.value.includes(value)
  }

  matchedLabelsCompletion.value = labelsCompletions.filter(matcher).map((item) => {
    const label = createLabel(item.key, item.value)

    label.id = item.value === '' ? `has label: ${label.id}` : label.id

    return label
  })
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
      v-on-click-outside="() => (showCompletions = false)"
      class="h-full flex-1 flex-wrap text-xs"
      icon="search"
      :model-value="filterValue"
      :on-clear="
        () => {
          $emit('update:filter-labels', [])
        }
      "
      :placeholder
      @update:model-value="(value) => $emit('update:filter-value', value)"
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
            :label="{
              ...label,
              removable: true,
            }"
            :remove-label="
              async () => {
                removeLabel(index)
              }
            "
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
        <ItemLabel :label="suggestion" class="pointer-events-none" />
      </div>
    </div>
  </div>
</template>
