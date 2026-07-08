<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import Card from '@/components/Card/Card.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import { useOngoingTasks } from '@/methods/ongoingTasks'
import { formatISO } from '@/methods/time'

const { data } = useOngoingTasks()
</script>

<template>
  <Card v-if="data.length" class="text-xs">
    <header class="flex items-center justify-between gap-1 px-4 py-3">
      <h2 class="text-sm font-medium text-naturals-n14">Ongoing Operations</h2>
      <span class="text-naturals-n11">{{ data.length }}</span>
    </header>

    <div
      v-for="{ item, summary } in data"
      :key="item.metadata.id"
      class="flex items-center gap-3 border-t border-naturals-n4 px-4 py-3"
    >
      <TIcon
        icon="loading"
        class="size-4 shrink-0 animate-spin text-primary-p3"
        aria-hidden="true"
      />

      <span class="min-w-0 grow truncate font-medium text-naturals-n14">
        {{ item.spec.title }}
      </span>

      <span class="min-w-0 truncate text-naturals-n11">{{ summary }}</span>

      <span v-if="item.metadata.created" class="shrink-0 text-naturals-n9 tabular-nums">
        {{ formatISO(item.metadata.created, 'HH:mm:ss') }}
      </span>
    </div>
  </Card>
</template>
