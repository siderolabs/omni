<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterStatusType, DefaultNamespace } from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import Card from '@/components/common/Card/Card.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'

const { data } = useResourceWatch<ClusterStatusSpec>({
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
  },
  runtime: Runtime.Omni,
  sortByField: 'created',
  sortDescending: true,
})
</script>

<template>
  <Card class="text-xs">
    <header class="flex justify-between gap-1 px-4 py-3">
      <h2 class="text-sm font-medium text-naturals-n14">Recent Clusters</h2>

      <TButton
        icon-position="left"
        icon="clusters"
        variant="subtle"
        size="xs"
        @click="$router.push({ name: 'Clusters' })"
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
          :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id } }"
          class="list-item-link truncate"
        >
          {{ item.metadata.id }}
        </RouterLink>

        <CopyButton :text="item.metadata.id" />
      </div>

      <div class="flex min-w-0 justify-center">
        <span class="truncate">
          {{ item.spec.machines?.total }}
          {{ pluralize('Node', item.spec.machines?.total) }}
        </span>
      </div>

      <ClusterStatus :cluster="item" class="place-self-end" />
    </div>
  </Card>
</template>
