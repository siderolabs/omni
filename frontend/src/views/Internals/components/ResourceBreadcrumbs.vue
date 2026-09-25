<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import { MetaNamespace, ResourceDefinitionType } from '@/api/resources'
import TSelectList, { type SelectItemObject } from '@/components/SelectList/TSelectList.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import type { ResourceDefinitionSpec } from '@/views/Internals/lib/resourceDefinition'
import { useMachineOptions } from '@/views/Internals/lib/useMachineOptions'
import {
  type ResourceRuntime,
  type ResourceTarget,
  useResourceRuntime,
} from '@/views/Internals/lib/useResourceRuntime'

const { runtime, machine, type } = defineProps<{
  runtime: ResourceRuntime
  /** The Talos machine being browsed. Without one this is the machine list. */
  machine?: string
  /** The full resource type being viewed. Without one this is the resource type list. */
  type?: string
}>()

const router = useRouter()

const target = computed<ResourceTarget | undefined>(() => {
  if (runtime === 'omni') return { runtime }
  if (machine) return { runtime, machine }

  return undefined
})

const { watchOptions, listRoute, resourceRoute } = useResourceRuntime(
  () => target.value ?? { runtime: 'omni' },
)

const { machines } = useMachineOptions(() => runtime !== 'talos')

const machineItems = computed<SelectItemObject<string>[]>(() =>
  machines.value.map(({ id, name, location, connected }) => ({
    label: location ? `${name} (${location})` : name,
    value: id,
    disabled: !connected,
    tooltip: connected ? undefined : 'Machine is not connected',
  })),
)

const { data: definitions } = useResourceWatch<ResourceDefinitionSpec>(() => ({
  ...watchOptions.value,
  skip: !type || !target.value,
  resource: {
    namespace: MetaNamespace,
    type: ResourceDefinitionType,
  },
}))

// Labelled by display name, but valued by the full type since that is what routes use.
const typeItems = computed<SelectItemObject<string>[]>(() => {
  const items = definitions.value
    .map(({ spec }) => ({ label: spec.displayType || spec.type!, value: spec.type! }))
    .sort((a, b) => a.label.localeCompare(b.label))

  // Keep the current type selectable while definitions load, or if it doesn't exist.
  return type && !items.some((i) => i.value === type)
    ? [{ label: type, value: type }, ...items]
    : items
})

function selectMachine(value?: string) {
  if (!value || value === machine) return

  // Talos resource types are the same across nodes, so stay on the current type.
  router.push(
    type
      ? { name: 'InternalsTalosResource', params: { machine: value, type } }
      : { name: 'InternalsTalosResources', params: { machine: value } },
  )
}

function selectType(value?: string) {
  if (value && value !== type) router.push(resourceRoute(value))
}
</script>

<template>
  <nav aria-label="Breadcrumb" class="flex flex-wrap items-center">
    <RouterLink
      v-if="type"
      class="p-2 leading-none font-medium text-naturals-n14 transition hover:opacity-50"
      :to="listRoute"
    >
      Resources
    </RouterLink>

    <span v-else class="p-2 leading-none font-medium text-naturals-n14">Resources</span>

    <template v-if="runtime === 'talos'">
      <svg
        class="size-5 shrink-0 opacity-50"
        xmlns="http://www.w3.org/2000/svg"
        fill="currentColor"
        viewBox="0 0 20 20"
        aria-hidden="true"
      >
        <path d="M5.555 17.776l8-16 .894.448-8 16-.894-.448z" />
      </svg>

      <TSelectList
        :model-value="machine"
        variant="breadcrumb"
        :values="machineItems"
        placeholder="Select machine"
        searcheable
        @update:model-value="selectMachine"
      />
    </template>

    <template v-if="type">
      <svg
        class="size-5 shrink-0 opacity-50"
        xmlns="http://www.w3.org/2000/svg"
        fill="currentColor"
        viewBox="0 0 20 20"
        aria-hidden="true"
      >
        <path d="M5.555 17.776l8-16 .894.448-8 16-.894-.448z" />
      </svg>

      <TSelectList
        :model-value="type"
        variant="breadcrumb"
        :values="typeItems"
        searcheable
        @update:model-value="selectType"
      />
    </template>
  </nav>
</template>
