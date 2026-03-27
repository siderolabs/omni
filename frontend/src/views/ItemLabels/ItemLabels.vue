<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import type { Resource } from '@/api/grpc'
import { SystemLabelPrefix } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import type { Label } from '@/methods/labels'
import { getLabelFromID } from '@/methods/labels'
import { showError } from '@/notification'

import ItemLabel from './ItemLabel.vue'

const { resource, addLabelFunc, removeLabelFunc } = defineProps<{
  resource: Resource
  addLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
  removeLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
}>()

defineEmits<{
  filterLabel: [Label]
}>()

const labelOrder = {
  'is-managed-by-static-infra-provider': -1,
  'machine-request-set': -1,
  'no-manual-allocation': -1,
  'machine-request': -1,
  installed: -1,
  'ready-to-use': -1,
  'reporting-events': -1,
  'invalid-state': 0,
  cluster: 0,
  'role-controlplane': 2,
  'role-worker': 2,
  available: 5,
  connected: 10,
  disconnected: 10,
  platform: 20,
  cores: 30,
  mem: 40,
  storage: 50,
  net: 60,
  cpu: 70,
  arch: 80,
  region: 100,
  zone: 110,
  instance: 120,
  'talos-version': 130,
}

const getLabelOrder = (l: Label) => {
  return labelOrder[l.id as keyof typeof labelOrder] ?? 1000
}

const labels = computed(() => {
  const labels = resource.metadata.labels || {}

  return Object.keys(labels)
    .map((key) => getLabelFromID(key, labels[key]))
    .filter((label) => getLabelOrder(label) !== -1)
    .sort((a, b) => getLabelOrder(a) - getLabelOrder(b))
})

const addingLabel = ref(false)
const currentLabel = ref('')
let addPending = false

const editLabels = () => {
  addingLabel.value = true
}

const addUserLabel = async () => {
  if (
    !addingLabel.value ||
    !currentLabel.value.trim() ||
    !resource.metadata.id ||
    addPending ||
    !addLabelFunc
  ) {
    return
  }

  try {
    addPending = true
    await addLabelFunc(resource.metadata.id, currentLabel.value)
  } catch (e) {
    showError(`Failed to Add Label ${currentLabel.value}`, e.message)
  }

  addingLabel.value = false

  currentLabel.value = ''
  addPending = false
}

const destroyUserLabel = async (key: string) => {
  if (!resource.metadata.id || !removeLabelFunc) {
    return
  }

  if (key.startsWith(SystemLabelPrefix)) {
    showError(
      `Failed to Remove Label ${key}`,
      `Label ${key} is not a user label and cannot be removed.`,
    )

    return
  }

  try {
    await removeLabelFunc(resource.metadata.id, key)
  } catch (e) {
    showError(`Failed to Remove Label ${key}`, e.message)
  }

  addingLabel.value = false

  currentLabel.value = ''
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-1.5 text-xs">
    <ItemLabel
      v-for="label in labels"
      :key="label.key"
      :label="label"
      :remove-label="removeLabelFunc ? destroyUserLabel : undefined"
      @filter-label="(label) => $emit('filterLabel', label)"
    />
    <TInput
      v-if="addingLabel"
      v-model="currentLabel"
      compact
      :focus="addingLabel"
      class="h-6 w-24"
      @keydown.enter="addUserLabel"
      @click.stop
      @blur="addUserLabel"
    />
    <TButton v-else-if="addLabelFunc" icon="tag" size="sm" @click.stop="editLabels">
      new label
    </TButton>
  </div>
</template>
