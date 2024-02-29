<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="menu">
    <Listbox v-model="selectedItem">
      <ListboxButton class="menu-button flex items-center gap-1">
        <div class="flex overflow-hidden">
          <div class="menu-title">{{ title }}:</div><div class="flex-1 truncate">{{ selectedItem }}</div>
        </div>
        <t-icon class="menu-arrow" icon="arrow-down" />
      </ListboxButton>
      <t-animation @after-enter="focusSearch = true" @after-leave="focusSearch = false">
        <ListboxOptions class="menu-items">
          <t-input @keydown.stop="() => {}" :focus="focusSearch" icon="search" v-if="searcheable" title="" class="search-box" placeholder="Search"
            v-model="searchTerm"/>
          <div class="menu-items-wrapper" v-if="filteredValues.length > 0">
            <div @click="$emit('checkedValue', item)" v-for="(item, idx) in filteredValues" :key="idx">
              <ListboxOption v-slot="{ active, selected }" class="menu-item" :value="item">
                <div class="menu-item-wrapper">
                  <span class="menu-item-text" :class="{ active: active }">
                    <word-highligher :query="searchTerm" :textToHighlight="item.toString()" highlightClass="text-naturals-N14 font-medium bg-transparent"/>
                  </span>
                  <t-icon icon="check" class="menu-check-icon" v-show="selected" />
                </div>
              </ListboxOption>
            </div>
          </div>
        </ListboxOptions>
      </t-animation>
    </Listbox>
  </div>
</template>

<script setup lang="ts">
import {
  Listbox,
  ListboxButton,
  ListboxOptions,
  ListboxOption,
} from "@headlessui/vue";

import { ref, toRefs, computed } from "vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TAnimation from "@/components/common/Animation/TAnimation.vue";
import WordHighligher from "vue-word-highlighter";

const props = defineProps<{
  title?: string,
  defaultValue?: (string | number),
  values: (string | number)[],
  searcheable?: Boolean,
}>();

const emit = defineEmits(["checkedValue"]);

const { values } = toRefs(props);
const searchTerm = ref("");
const selectedItem = ref(props.defaultValue);
const focusSearch = ref(false);

defineExpose({
  selectItem: (value: string) => {
    selectedItem.value = value;
    emit("checkedValue", value);
  }
});

const filteredValues = computed(() => {
  if (!searchTerm.value) {
    return values.value;
  }

  const term = searchTerm.value.toLowerCase();

  return values.value.filter(item => item.toString().toLowerCase().indexOf(term) != -1);
});
</script>

<style scoped>
.menu {
  @apply relative;
}

.menu-button {
  @apply w-full h-full bg-naturals-N2 rounded border border-naturals-N7 flex justify-between items-center text-xs text-naturals-N14;
  padding: 9px 12px;
}

.menu-title {
  @apply text-xs mr-1 whitespace-nowrap truncate;
}

.menu-arrow {
  @apply fill-current transition-all duration-300;
  width: 16px;
  height: 16px;
}

.menu-items {
  @apply flex flex-col rounded bg-naturals-N3 border border-naturals-N4 absolute top-10 left-0 min-w-full z-10 gap-1 p-1.5;
  max-height: 280px;
}

.search-box {
  @apply text-xs text-naturals-N9 h-8;
}

.menu-item {
  @apply text-xs text-naturals-N9 cursor-pointer;
  padding: 6px 12px;
}

.menu-item-wrapper {
  @apply flex justify-between items-center;
}

.menu-item-text {
  @apply transition-all truncate;
  margin-right: 8px;
}

.active {
  @apply text-naturals-N13;
}

.menu-items-wrapper {
  @apply flex flex-col overflow-auto py-2;
}

.menu-check-icon {
  @apply w-3 h-3 fill-current text-naturals-N14;
}
</style>
