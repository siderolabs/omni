<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-wrap gap-1.5 items-center text-xs">
    <item-label
      @filterLabel="(label) => $emit('filterLabel', label)"
      v-for="label in labels" :label="label" :key="label.key" :remove-label="removeLabelFunc ? destroyUserLabel : undefined"/>
    <t-input @keydown.enter="addUserLabel" v-model="currentLabel" compact @click.stop @blur="addUserLabel" :focus="addingLabel" v-if="addingLabel" class="w-24 h-6"/>
    <t-button icon="tag" type="compact" @click.stop="editLabels" v-else-if="addLabelFunc">new label</t-button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRefs } from "vue";
import {Resource} from "@/api/grpc";

import ItemLabel from "./ItemLabel.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import { showError } from "@/notification";
import { SystemLabelPrefix } from "@/api/resources";
import { Label, getLabelFromID } from "@/methods/labels";

const props = defineProps<{
  resource: Resource
  addLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
  removeLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
}>();

const { resource } = toRefs(props);

defineEmits(['filterLabel']);

const labelOrder = {
  "is-managed-by-static-infra-provider": -1,
  "machine-request-set": -1,
  "no-manual-allocation": -1,
  "machine-request": -1,
  "installed": -1,
  "ready-to-use": -1,
  "reporting-events": -1,
  "invalid-state": 0,
  "cluster": 0,
  "role-controlplane": 2,
  "role-worker": 2,
  "available": 5,
  "connected": 10,
  "disconnected": 10,
  "platform": 20,
  "cores": 30,
  "mem": 40,
  "storage": 50,
  "net": 60,
  "cpu": 70,
  "arch": 80,
  "region": 100,
  "zone": 110,
  "instance": 120,
  "talos-version": 130,
}

const getLabelOrder = (l: Label) => {
  return labelOrder[l.id] ?? 1000;
}

const labels = computed((): Array<Label> => {
  const labels = resource.value.metadata?.labels || {};

  let labelsArray: Array<Label> = [];

  for (const key in labels) {
    const label = getLabelFromID(key, labels[key]);

    if (getLabelOrder(label) === -1) {
      continue;
    }

    labelsArray.push(label);
  }

  labelsArray.sort((a, b) => {
    if (getLabelOrder(a) === getLabelOrder(b)) {
      return 0;
    }

    if (getLabelOrder(a) > getLabelOrder(b)) {
      return 1;
    }

    return -1;
  });

  return labelsArray
});

const addingLabel = ref(false);
const currentLabel = ref("");
let addPending = false;

const editLabels = () => {
  addingLabel.value = true;
};

const addUserLabel = async () => {
  if (!addingLabel.value || !currentLabel.value.trim() || !resource.value.metadata.id || addPending || !props.addLabelFunc) {
    return;
  }

  try {
    addPending = true;
    await props.addLabelFunc(resource.value.metadata.id, currentLabel.value);
  } catch (e) {
    showError(`Failed to Add Label ${currentLabel.value}`, e.message);
  }

  addingLabel.value = false;

  currentLabel.value = "";
  addPending = false;
}

const destroyUserLabel = async (key: string) => {
  if (!resource.value.metadata.id || !props.removeLabelFunc) {
    return;
  }

  if (key.startsWith(SystemLabelPrefix)) {
    showError(
        `Failed to Remove Label ${key}`,
        `Label ${key} is not a user label and cannot be removed.`)

    return;
  }

  try {
    await props.removeLabelFunc(resource.value.metadata.id, key);
  } catch (e) {
    showError(`Failed to Remove Label ${key}`, e.message);
  }

  addingLabel.value = false;

  currentLabel.value = "";
};
</script>
