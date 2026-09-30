<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import type { Resource } from '@/api/grpc'
import { SystemLabelPrefix } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { Label } from '@/methods/labels'
import { getLabelFromID } from '@/methods/labels'
import { showError } from '@/notification'

import ItemLabel from './ItemLabel.vue'

const { resource, addLabelFunc, removeLabelFunc, hideConnection } = defineProps<{
  resource: Resource
  hideConnection?: boolean
  addLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
  removeLabelFunc?: (resourceID: string, ...labels: string[]) => Promise<void> | void
}>()

defineEmits<{
  selectLabel: [label: Label]
}>()

const hidden = new Set([
  'is-managed-by-static-infra-provider',
  'machine-request-set',
  'no-manual-allocation',
  'machine-request',
  'installed',
  'ready-to-use',
  'reporting-events',
])

// Labels render in groups, in this order, with a wider gap between groups.
// Within a group they keep the order listed here.
const labelGroups = [
  ['invalid-state'],
  ['cluster', 'role-controlplane', 'role-worker', 'available', 'machine-set'],
  ['connected', 'disconnected'],
  ['enterprise', 'fips', 'talos-version'],
  ['platform', 'cores', 'mem', 'storage', 'net', 'cpu', 'arch', 'region', 'zone', 'instance'],
]

const systemGroup = labelGroups.length
const userGroup = labelGroups.length + 1

const position = (l: Label): [number, number] => {
  for (const [group, ids] of labelGroups.entries()) {
    const index = ids.indexOf(l.id)

    if (l.system && index !== -1) return [group, index]
  }

  return [l.system ? systemGroup : userGroup, 0]
}

const groups = computed(() => {
  const labels = resource.metadata.labels || {}
  const result: Label[][] = []

  Object.keys(labels)
    .map((key) => getLabelFromID(key, labels[key]))
    .filter((label) => !hidden.has(label.id))
    .filter(
      (label) =>
        !hideConnection || !(label.system && ['connected', 'disconnected'].includes(label.id)),
    )
    .map((label) => ({ label, pos: position(label) }))
    .sort((a, b) => a.pos[0] - b.pos[0] || a.pos[1] - b.pos[1])
    .forEach(({ label, pos }) => (result[pos[0]] ??= []).push(label))

  return result.filter(Boolean)
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
  <div class="flex flex-wrap items-center gap-x-6 gap-y-1.5 text-xs">
    <div v-for="(group, i) in groups" :key="i" class="flex flex-wrap items-center gap-1.5">
      <ItemLabel
        v-for="label in group"
        :key="label.key"
        :label="{ ...label, removable: label.removable && !!removeLabelFunc }"
        @remove-label="removeLabelFunc ? destroyUserLabel(label.key) : undefined"
        @select-label="$emit('selectLabel', label)"
      />
    </div>
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
