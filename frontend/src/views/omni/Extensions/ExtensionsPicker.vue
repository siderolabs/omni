<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2 overflow-hidden">
    <t-input icon="search" v-model="filterExtensions"/>

    <div class="grid grid-cols-4 bg-naturals-N4 uppercase text-xs text-naturals-N13 pl-2 py-2">
      <div class="col-span-2">Name</div>
      <div>Version</div>
      <div>Author</div>
    </div>

    <Watch
      class="flex-1 overflow-y-auto overflow-x-hidden"
      v-if="talosVersion"
      :opts="{
        resource: {
          id: talosVersion,
          type: TalosExtensionsType,
          namespace: DefaultNamespace,
        },
        runtime: Runtime.Omni,
      }"
      displayAlways
      >
      <template #default="{ items }">
        <div v-if="items[0]?.spec.items" class="flex flex-col">
          <div v-for="extension in filteredExtensions(items[0].spec.items!)" :key="extension.name" class="cursor-pointer flex gap-2 hover:bg-naturals-N5 transition-colors p-2 border-b border-naturals-N6 items-center"
              @click="() => updateExtension(extension)">
            <t-checkbox
                :icon="(indeterminate && !changed) ? 'minus' : 'check'"
                class="col-span-2"
                :disabled="immutableExtensions?.[extension.name!]"
                :checked="modelValue[extension.name!]"/>
            <div class="grid grid-cols-4 gap-1 flex-1 items-center">
              <div class="text-xs text-naturals-N13 col-span-2">
                <WordHighlighter
                    :query="filterExtensions"
                    :textToHighlight="extension.name!.slice('siderolabs/'.length)"
                    highlightClass="bg-naturals-N14"
                />
              </div>
              <div class="text-xs text-naturals-N13">{{ extension.version }}</div>
              <div class="text-xs text-naturals-N13">{{ extension.author }}</div>
              <div class="col-span-4 text-xs" v-if="extension.description && showDescriptions">
                {{ extension.description }}
              </div>
            </div>
          </div>
        </div>
        <div v-else class="flex gap-1 items-center text-xs p-4 text-primary-P2">
          <t-icon class="w-3 h-3" icon="warning"/>No extensions available for this Talos version
        </div>
      </template>
    </Watch>
  </div>
</template>

<script setup lang="ts">
import { ref, toRefs } from 'vue';
import { TalosExtensionsSpecInfo } from "@/api/omni/specs/omni.pb";
import { DefaultNamespace, TalosExtensionsType } from '@/api/resources';
import { Runtime } from '@/api/common/omni.pb';

import TInput from '@/components/common/TInput/TInput.vue';
import TIcon from '@/components/common/Icon/TIcon.vue';
import WordHighlighter from "vue-word-highlighter";
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue';
import Watch from '@/components/common/Watch/Watch.vue';

const emit = defineEmits(["update:model-value"]);

const props = defineProps<{
  talosVersion: string
  modelValue: Record<string, boolean>
  immutableExtensions?: Record<string, boolean>
  indeterminate?: boolean
}>();

const { modelValue } = toRefs(props);
const filterExtensions = ref<string>("");
const showDescriptions = ref(false);

const filteredExtensions = (items: TalosExtensionsSpecInfo[]) => {
  if (!filterExtensions.value) {
    return items;
  }

  return items.filter(item => item.name?.includes(filterExtensions.value));
}

let changed = ref(false);

const updateExtension = (extension: TalosExtensionsSpecInfo) => {
  const newState = {
    ...modelValue.value,
  }

  newState[extension.name!] = !newState[extension.name!];

  emit("update:model-value", newState);

  changed.value = true;
};
</script>
