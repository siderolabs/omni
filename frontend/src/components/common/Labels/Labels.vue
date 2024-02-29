<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-wrap gap-1.5 items-center text-xs">
    <span v-for="label, key in modelValue" :key="key" class="flex items-center cursor-pointer" v-bind:class="`resource-label label-${label.color ?? 'light6'}`">
        <template v-if="label.value">
        {{ key }}:<span class="font-semibold">{{ label.value }}</span>
        </template>
        <span v-else class="font-semibold">
        {{ key }}
        </span>
        <t-icon v-if="label.canRemove" icon="close" class="destroy-label-button" @click.stop="() => removeLabel(key)"/>
    </span>
    <t-input @keydown.enter="addLabel" v-model="currentLabel" compact @click.stop @blur="addingLabel = false" :focus="addingLabel" v-if="addingLabel" class="w-24 h-6"/>
    <t-button icon="tag" type="compact" @click.stop="editLabels" v-else>new label</t-button>
  </div>
</template>

<script setup lang="ts">
import { ref, toRefs } from "vue";

import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";

type Label = {
  value: string
  canRemove: boolean
  color?: string
}

const props = defineProps<{
  modelValue: Record<string, Label>
  onAdd?: (value: string) => Promise<void>
  onRemove?: (value: string) => Promise<void>
}>();

const { modelValue } = toRefs(props)
const emit = defineEmits(['update:modelValue']);

const addingLabel = ref(false);
const currentLabel = ref("");

const editLabels = () => {
  addingLabel.value = true;
};

const addLabel = async () => {
  addingLabel.value = false;

  const parts = currentLabel.value.split(":");

  currentLabel.value = "";

  emit("update:modelValue", {...modelValue.value,
    [parts[0]]: {
      value: parts[1] ?? "",
      canRemove: true,
    }
  });
}

const removeLabel = async (key: string) => {
  addingLabel.value = false;

  currentLabel.value = "";

  const copied = {...modelValue.value};

  delete copied[key];

  emit("update:modelValue", copied);
};
</script>

<style>
.destroy-label-button {
  @apply w-3 h-3 -mr-1 ml-1 inline-block hover:text-naturals-N1 cursor-pointer hover:bg-naturals-N14 transition-all rounded-full;
}
</style>
