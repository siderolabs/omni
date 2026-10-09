<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { LabelCluster } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import Card from '@/components/Card/Card.vue'
import CopyButton from '@/components/CopyButton/CopyButton.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import MachineStage from '@/components/Status/MachineStage.vue'
import { getMachineName } from '@/methods/node'

defineProps<{
  machines: Resource<MachineStatusLinkSpec>[]
  loading: boolean
}>()
</script>

<template>
  <Card class="text-xs">
    <header class="flex justify-between gap-micro px-compact py-snug">
      <h2 class="text-sm font-medium text-content-emphasis">Recent Machines</h2>

      <TButton
        icon-position="left"
        icon="nodes"
        variant="subtle"
        size="xs"
        @click="$router.push({ name: 'Machines' })"
      >
        View All
      </TButton>
    </header>

    <div v-if="!machines.length" class="p-compact">
      <TSpinner v-if="loading" class="mx-auto size-4" />
      <span v-else>No machines found</span>
    </div>

    <div
      v-for="item in machines.slice(0, 5)"
      :key="item.metadata.id"
      class="grid grid-cols-3 items-center gap-tight border-t border-border-default px-compact py-snug max-sm:grid-cols-[1fr_1fr_auto]"
    >
      <div class="flex min-w-0 items-center gap-tight">
        <RouterLink
          :to="{ name: 'Machine', params: { machine: item.metadata.id! } }"
          class="list-item-link truncate"
        >
          {{ getMachineName(item) }}
        </RouterLink>

        <CopyButton :text="getMachineName(item)" />
      </div>

      <div class="flex min-w-0 justify-center">
        <span v-if="item.metadata.labels?.[LabelCluster]" class="resource-label truncate">
          <span class="text-content-secondary">cluster:</span>
          <span>{{ item.metadata.labels[LabelCluster] }}</span>
        </span>
      </div>

      <MachineStage :machine="item" class="place-self-end" />
    </div>
  </Card>
</template>
