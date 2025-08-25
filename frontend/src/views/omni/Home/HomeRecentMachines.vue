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
import { useWatch } from '@/components/common/Watch/useWatch'

const { items } = useWatch<MachineStatusSpec>({
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
    <header class="flex justify-between gap-1">
      <h2 class="text-sm text-naturals-n14">Recent Machines</h2>

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
      v-for="item in items.slice(0, 5)"
      :key="itemID(item)"
      class="flex w-full place-content-between px-6 py-4 not-first-of-type:border-t not-first-of-type:border-naturals-n4"
    >
      <div class="grid flex-1 grid-cols-3">
        <RouterLink
          :to="{ name: 'Machine', params: { machine: item.metadata.id } }"
          class="list-item-link"
        >
          {{ item.metadata.id }}
        </RouterLink>

        <div>
          {{ item.spec.cluster }}
        </div>
      </div>
    </div>
  </Card>
</template>
