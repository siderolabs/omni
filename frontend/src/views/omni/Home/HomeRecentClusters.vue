<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import Card from '@/components/common/Card/Card.vue'
import CopyButton from '@/components/common/CopyButton/CopyButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import ClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'

defineProps<{
  clusters: Resource<ClusterStatusSpec>[]
  loading: boolean
}>()
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

    <div v-if="!clusters.length" class="p-4">
      <TSpinner v-if="loading" class="mx-auto size-4" />
      <span v-else>No clusters found</span>
    </div>

    <div
      v-for="item in clusters.slice(0, 5)"
      :key="itemID(item)"
      class="grid grid-cols-3 items-center gap-2 border-t border-naturals-n4 px-4 py-3 max-sm:grid-cols-[1fr_1fr_auto]"
    >
      <div class="flex min-w-0 items-center gap-2">
        <RouterLink
          :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id! } }"
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
