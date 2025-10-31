<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends string | number">
import { useMounted } from '@vueuse/core'
import {
  Label,
  SelectContent,
  SelectIcon,
  SelectItem,
  SelectItemIndicator,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectTrigger,
  SelectValue,
  SelectViewport,
} from 'reka-ui'
import { computed, onBeforeMount, ref, useId } from 'vue'
import WordHighligher from 'vue-word-highlighter'

import TIcon from '@/components/common/Icon/TIcon.vue'
import TInput from '@/components/common/TInput/TInput.vue'

const {
  title = '',
  defaultValue = undefined,
  values,
  searcheable,
} = defineProps<{
  title?: string
  defaultValue?: T
  values: T[]
  searcheable?: boolean
  hideSelectedSmallScreens?: boolean
  overheadTitle?: boolean
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
  selectItem: (value: T) => {
    selectedItem.value = value
  },
})

// Passing this as a default option to defineModel doesn't work due to the macro's limitations
onBeforeMount(() => {
  if (
    typeof defaultValue !== 'undefined' &&
    typeof selectedItem.value === 'undefined' &&
    selectedItem.value !== defaultValue
  ) {
    selectedItem.value = defaultValue
  }
})

const filteredValues = computed(() => {
  if (!searchTerm.value) {
    return values
  }

  const term = searchTerm.value.toLowerCase()

  return values.filter((item) => item.toString().toLowerCase().includes(term))
})

// Focus is handled by reka-ui, using timeout to skip their focusing logic in this case
const onOpen = async (open: boolean) => {
  if (!searcheable) return

  setTimeout(() => (focusSearch.value = open))
}
</script>

<template>
  <component :is="title && overheadTitle ? Label : 'div'" class="inline-block">
    <span
      v-if="title && overheadTitle"
      class="mb-4 inline-block text-sm font-medium text-naturals-n14"
    >
      {{ title }}
    </span>

    <SelectRoot v-model="selectedItem" @update:open="onOpen">
      <SelectTrigger
        :id="triggerId"
        class="flex size-full items-center justify-between gap-1 rounded border border-naturals-n7 bg-naturals-n2 px-3 py-2.25 text-xs text-naturals-n14"
      >
        <SelectValue class="flex gap-1 truncate select-none">
          <Label
            v-if="title && !overheadTitle"
            :for="triggerId"
            aria-hidden="true"
            :class="hideSelectedSmallScreens ? `md:after:content-[':']` : `after:content-[':']`"
          >
            {{ title }}
          </Label>
          <span :class="{ 'max-md:hidden': hideSelectedSmallScreens }">
            {{ selectedItem }}
          </span>
        </SelectValue>
        <SelectIcon>
          <TIcon class="size-4 fill-current transition-all duration-300" icon="arrow-down" />
        </SelectIcon>
      </SelectTrigger>

      <SelectPortal>
        <SelectContent
          class="relative z-50 max-h-[min(--spacing(70),var(--reka-select-content-available-height))] min-w-(--reka-select-trigger-width) translate-y-1 space-y-1 overflow-hidden rounded border border-naturals-n4 bg-naturals-n3 p-1.5 text-xs slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
          position="popper"
          side="bottom"
        >
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
            <TIcon icon="arrow-up" class="mx-auto size-4" />
          </SelectScrollUpButton>

          <SelectViewport>
            <SelectItem
              v-for="item in filteredValues"
              :key="item"
              class="flex cursor-pointer items-center justify-between gap-2 px-3 py-1.5 text-naturals-n9 outline-none hover:text-naturals-n13 focus:text-naturals-n13 data-[state=checked]:text-naturals-n13"
              :value="item"
            >
              <SelectItemText class="truncate transition-all">
                <WordHighligher
                  :query="searchTerm"
                  :text-to-highlight="item.toString()"
                  highlight-class="text-naturals-n14 font-medium bg-transparent truncate"
                />
              </SelectItemText>

              <SelectItemIndicator>
                <TIcon icon="check" class="size-3" />
              </SelectItemIndicator>
            </SelectItem>
          </SelectViewport>

          <SelectScrollDownButton>
            <TIcon icon="arrow-down" class="mx-auto size-4" />
          </SelectScrollDownButton>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>
  </component>
</template>
