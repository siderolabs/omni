<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import Card from '@/components/common/Card/Card.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { useWatch } from '@/components/common/Watch/useWatch'

const { data } = useWatch<MachineStatusSpec>({
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  runtime: Runtime.Omni,
  sortByField: 'created',
  sortDescending: true,
})
</script>

<template>
  <Card class="text-xs">
    <header class="flex justify-between gap-1 px-4 py-3">
      <h2 class="text-sm font-medium text-naturals-n14">Recent Machines</h2>

      <TButton
        icon-position="left"
        icon="nodes"
        type="subtle"
        @click="$router.push({ name: 'Machines' })"
      >
        View All
      </TButton>
    </header>

    <div
      v-for="item in data.slice(0, 5)"
      :key="itemID(item)"
      class="grid grid-cols-3 items-center gap-2 border-t border-naturals-n4 px-4 py-3 max-sm:grid-cols-[1fr_1fr_auto]"
    >
      <div class="flex min-w-0 items-center gap-2">
        <RouterLink
          :to="{ name: 'Machine', params: { machine: item.metadata.id } }"
          class="list-item-link truncate"
        >
          {{ item.metadata.id }}
        </RouterLink>

        <CopyButton :text="item.metadata.id" />
      </div>

      <div class="flex min-w-0 justify-center">
        <span v-if="item.spec.cluster" class="resource-label label-light1 truncate">
          cluster:
          <span class="font-semibold">{{ item.spec.cluster }}</span>
        </span>
      </div>

      <div
        class="flex items-center gap-1 place-self-end"
        :class="item.metadata.phase === 'running' ? 'text-green-g1' : 'text-red-r1'"
      >
        <TIcon
          class="h-4"
          :icon="item.metadata.phase === 'running' ? 'check-in-circle' : 'delete'"
        />

        <span class="contents max-sm:hidden">
          {{ item.metadata.phase === 'running' ? 'Running' : 'Destroying' }}
        </span>
      </div>
    </div>
  </Card>
</template>
