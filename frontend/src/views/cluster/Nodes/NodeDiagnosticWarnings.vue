<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col flex-1">
    <t-list-item v-for="diagnostic in diagnostics" :key="diagnostic.id" disable-border-on-expand>
      <a
        class="diagnostic-item"
        :href="'https://talos.dev/diagnostic/' + diagnostic.id"
        target="_blank"
      >
        <t-icon icon="warning" class="w-4 h-4 text-yellow-400 mr-2" />
        <div class="text-yellow-400">{{ diagnostic.message }}</div>
      </a>
      <template #details>
        <div class="diagnostic-sublist">
          <div
            class="diagnostic-subitem"
            v-for="(detail, index) in diagnostic.details"
            :key="index"
          >
            <t-icon icon="dot" class="w-4 h-4 mr-2" />
            <span class="flex-1">{{ detail }}</span>
          </div>
        </div>
      </template>
    </t-list-item>
  </div>
</template>
<script setup lang="ts">
import type { MachineStatusSpecDiagnostic } from '@/api/omni/specs/omni.pb'
import TListItem from '@/components/common/List/TListItem.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  diagnostics?: MachineStatusSpecDiagnostic[]
}

defineProps<Props>()
</script>

<style scoped>
.diagnostic-item {
  @apply flex flex-row transition-colors hover:brightness-125;
}

.diagnostic-sublist {
  @apply pl-6 flex flex-col gap-1;
}

.diagnostic-subitem {
  @apply flex items-center text-naturals-N11;
}
</style>
