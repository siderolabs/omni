<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { MachineStatusSpecDiagnostic } from '@/api/omni/specs/omni.pb'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TListItem from '@/components/common/List/TListItem.vue'

type Props = {
  diagnostics?: MachineStatusSpecDiagnostic[]
}

defineProps<Props>()
</script>

<template>
  <div class="flex flex-1 flex-col">
    <TListItem v-for="diagnostic in diagnostics" :key="diagnostic.id" disable-border-on-expand>
      <a
        class="diagnostic-item"
        :href="'https://talos.dev/diagnostic/' + diagnostic.id"
        target="_blank"
        rel="noopener noreferrer"
      >
        <TIcon icon="warning" class="mr-2 h-4 w-4 text-yellow-400" />
        <div class="text-yellow-400">{{ diagnostic.message }}</div>
      </a>
      <template #details>
        <div class="diagnostic-sublist">
          <div
            v-for="(detail, index) in diagnostic.details"
            :key="index"
            class="diagnostic-subitem"
          >
            <TIcon icon="dot" class="mr-2 h-4 w-4" />
            <span class="flex-1">{{ detail }}</span>
          </div>
        </div>
      </template>
    </TListItem>
  </div>
</template>

<style scoped>
@reference "../../../../index.css";

.diagnostic-item {
  @apply flex flex-row transition-colors hover:brightness-125;
}

.diagnostic-sublist {
  @apply flex flex-col gap-1 pl-6;
}

.diagnostic-subitem {
  @apply flex items-center text-naturals-n11;
}
</style>
