<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Listbox, ListboxButton, ListboxOption, ListboxOptions } from '@headlessui/vue'
import { computed, ref, toRefs } from 'vue'
import WordHighligher from 'vue-word-highlighter'

import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TInput from '@/components/common/TInput/TInput.vue'

const props = withDefaults(
  defineProps<{
    title?: string
    defaultValue?: string | number
    values: (string | number)[]
    searcheable?: boolean
    menuAlign?: 'left' | 'right'
  }>(),
  {
    menuAlign: 'left',
  },
)

const emit = defineEmits(['checkedValue'])

const { values } = toRefs(props)
const searchTerm = ref('')
const selectedItem = ref(props.defaultValue)
const focusSearch = ref(false)

defineExpose({
  selectItem: (value: string) => {
    selectedItem.value = value
    emit('checkedValue', value)
  },
})

const filteredValues = computed(() => {
  if (!searchTerm.value) {
    return values.value
  }

  const term = searchTerm.value.toLowerCase()

  return values.value.filter((item) => item.toString().toLowerCase().indexOf(term) !== -1)
})
</script>

<template>
  <div class="menu">
    <Listbox v-model="selectedItem">
      <ListboxButton class="menu-button flex items-center gap-1">
        <div class="flex overflow-hidden">
          <div v-if="title" class="menu-title">{{ title }}:</div>
          <div class="flex-1 truncate">{{ selectedItem }}</div>
        </div>
        <TIcon class="menu-arrow" icon="arrow-down" />
      </ListboxButton>
      <TAnimation @after-enter="focusSearch = true" @after-leave="focusSearch = false">
        <ListboxOptions class="menu-items" :class="`${menuAlign}-0`">
          <TInput
            v-if="searcheable"
            v-model="searchTerm"
            :focus="focusSearch"
            icon="search"
            title=""
            class="search-box"
            placeholder="Search"
            @keydown.stop="() => {}"
          />
          <div v-if="filteredValues.length > 0" class="menu-items-wrapper">
            <div
              v-for="(item, idx) in filteredValues"
              :key="idx"
              @click="$emit('checkedValue', item)"
            >
              <ListboxOption v-slot="{ active, selected }" class="menu-item" :value="item">
                <div class="menu-item-wrapper">
                  <span class="menu-item-text" :class="{ active: active }">
                    <WordHighligher
                      :query="searchTerm"
                      :text-to-highlight="item.toString()"
                      highlight-class="text-naturals-N14 font-medium bg-transparent truncate"
                    />
                  </span>
                  <TIcon v-show="selected" icon="check" class="menu-check-icon" />
                </div>
              </ListboxOption>
            </div>
          </div>
        </ListboxOptions>
      </TAnimation>
    </Listbox>
  </div>
</template>

<style scoped>
.menu {
  @apply relative;
}

.menu-button {
  @apply flex h-full w-full items-center justify-between rounded border border-naturals-N7 bg-naturals-N2 text-xs text-naturals-N14;
  padding: 9px 12px;
}

.menu-title {
  @apply mr-1 truncate whitespace-nowrap text-xs;
}

.menu-arrow {
  @apply fill-current transition-all duration-300;
  width: 16px;
  height: 16px;
}

.menu-items {
  @apply absolute top-10 z-10 flex min-w-full flex-col gap-1 rounded border border-naturals-N4 bg-naturals-N3 p-1.5;
  max-height: 280px;
}

.search-box {
  @apply h-8 text-xs text-naturals-N9;
}

.menu-item {
  @apply cursor-pointer text-xs text-naturals-N9;
  padding: 6px 12px;
}

.menu-item-wrapper {
  @apply flex items-center justify-between;
}

.menu-item-text {
  @apply truncate transition-all;
  margin-right: 8px;
}

.active {
  @apply text-naturals-N13;
}

.menu-items-wrapper {
  @apply flex flex-col overflow-auto py-2;
}

.menu-check-icon {
  @apply h-3 w-3 fill-current text-naturals-N14;
}
</style>
