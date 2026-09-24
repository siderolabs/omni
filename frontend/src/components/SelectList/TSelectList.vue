<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface SelectItemObject<T extends string | number | undefined> {
  label: string
  value?: T
  disabled?: boolean
  tooltip?: string
}

export type SelectItemType<T extends string | number | undefined> = T | SelectItemObject<T>
</script>

<script setup lang="ts" generic="T extends string | number | undefined">
import { useMounted } from '@vueuse/core'
import { computed, onBeforeMount, ref, useId } from 'vue'
import WordHighligher from 'vue-word-highlighter'

import FormLabel from '@/components/FormLabel/FormLabel.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TInput from '@/components/TInput/TInput.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'

import SelectContent from './components/SelectContent.vue'
import SelectIcon from './components/SelectIcon.vue'
import SelectItem from './components/SelectItem.vue'
import SelectItemIndicator from './components/SelectItemIndicator.vue'
import SelectItemText from './components/SelectItemText.vue'
import SelectLabel from './components/SelectLabel.vue'
import SelectPortal from './components/SelectPortal.vue'
import SelectRoot from './components/SelectRoot.vue'
import SelectScrollDownButton from './components/SelectScrollDownButton.vue'
import SelectScrollUpButton from './components/SelectScrollUpButton.vue'
import SelectTrigger, { type SelectTriggerProps } from './components/SelectTrigger.vue'
import SelectValue from './components/SelectValue.vue'
import SelectViewport from './components/SelectViewport.vue'

const {
  variant,
  title = '',
  defaultValue = undefined,
  values,
  searcheable,
  placeholder,
} = defineProps<{
  variant?: SelectTriggerProps['variant']
  title?: string
  defaultValue?: T
  values: SelectItemType<T>[]
  disabled?: boolean
  searcheable?: boolean
  hideSelectedSmallScreens?: boolean
  overheadTitle?: boolean
  /** Shown in the trigger while nothing is selected. */
  placeholder?: string
}>()

const emit = defineEmits<{
  checkedValue: [T]
}>()

const isMounted = useMounted()
const searchTerm = ref('')
const selectedItem = defineModel<T>({
  set(v) {
    if (isMounted.value && v !== selectedItem.value) emit('checkedValue', v)

    return v
  },
})
const focusSearch = ref(false)
const triggerId = useId()

defineExpose({
  selectItem: (value?: T) => {
    selectedItem.value = value
  },
})

// Passing this as a default option to defineModel doesn't work due to the macro's limitations
onBeforeMount(() => {
  if (typeof defaultValue !== 'undefined' && typeof selectedItem.value === 'undefined') {
    selectedItem.value = defaultValue as T | undefined
  }
})

const filteredValues = computed(() => {
  if (!searchTerm.value) {
    return values
  }

  const term = searchTerm.value.toLowerCase()

  return values.filter((item) => itemLabel(item).toLowerCase().includes(term))
})

// Focus is handled by reka-ui, using timeout to skip their focusing logic in this case
const onOpen = async (open: boolean) => {
  if (!searcheable) return

  setTimeout(() => (focusSearch.value = open))
}

function itemLabel(item: T | { label: string }) {
  switch (typeof item) {
    case 'undefined':
      return '-'
    case 'string':
      return item
    case 'number':
      return item.toString()
    default:
      return item.label
  }
}

function itemValue(item?: SelectItemType<T>) {
  switch (typeof item) {
    case 'undefined':
      // undefined and '' are invalid values for reka-ui's Select
      return null
    case 'string':
    case 'number':
      return item
    default:
      return itemValue(item.value)
  }
}

function itemDisabled(item?: SelectItemType<T>) {
  return typeof item === 'object' && !!item.disabled
}

function itemTooltip(item?: SelectItemType<T>) {
  return typeof item === 'object' ? item.tooltip : undefined
}

function labelFromValue(value?: T | null) {
  // We translate undefined to null, translate it back here
  if (value === null) value = undefined

  const item = values.find((v) => {
    switch (typeof v) {
      case 'undefined':
      case 'string':
      case 'number':
        return value === v
      default:
        return value === v.value
    }
  })

  if (!item) return ''

  return itemLabel(item)
}
</script>

<template>
  <component :is="title && overheadTitle ? 'label' : 'div'" class="inline-block">
    <FormLabel v-if="title && overheadTitle" as="span" class="mb-4">
      {{ title }}
    </FormLabel>

    <SelectRoot v-model="selectedItem" :disabled @update:open="onOpen">
      <SelectTrigger :id="triggerId" :variant>
        <SelectValue>
          <SelectLabel
            v-if="title && !overheadTitle"
            :for="triggerId"
            aria-hidden="true"
            :hide-selected-small-screens
          >
            {{ title }}
          </SelectLabel>
          <span
            :class="{
              'max-md:hidden': hideSelectedSmallScreens,
              'text-naturals-n9': placeholder && !labelFromValue(selectedItem),
            }"
          >
            {{ labelFromValue(selectedItem) || placeholder }}
          </span>
        </SelectValue>
        <SelectIcon>
          <TIcon class="size-4 fill-current transition-all duration-300" icon="chevron-down" />
        </SelectIcon>
      </SelectTrigger>

      <SelectPortal>
        <SelectContent>
          <TInput
            v-if="searcheable"
            v-model="searchTerm"
            :focus="focusSearch"
            icon="search"
            aria-label="search"
            placeholder="Search"
            @keydown.stop="() => {}"
          />

          <SelectScrollUpButton>
            <TIcon icon="chevron-up" class="mx-auto size-(--arrow-size)" />
          </SelectScrollUpButton>

          <SelectViewport>
            <Tooltip
              v-for="item in filteredValues"
              :key="itemValue(item) ?? 'undefined'"
              :description="itemTooltip(item)"
              :disabled="!itemTooltip(item)"
              placement="right"
            >
              <SelectItem :value="itemValue(item)" :disabled="itemDisabled(item)">
                <span class="size-3">
                  <SelectItemIndicator as-child>
                    <TIcon icon="check" class="size-full" />
                  </SelectItemIndicator>
                </span>

                <SelectItemText>
                  <WordHighligher
                    :query="searchTerm"
                    :text-to-highlight="itemLabel(item)"
                    highlight-class="truncate bg-transparent font-medium text-naturals-n14"
                  />
                </SelectItemText>
              </SelectItem>
            </Tooltip>
          </SelectViewport>

          <SelectScrollDownButton>
            <TIcon icon="chevron-down" class="mx-auto size-(--arrow-size)" />
          </SelectScrollDownButton>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>
  </component>
</template>
