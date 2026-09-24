<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { dump } from 'js-yaml'
import { computed, ref, watch } from 'vue'

import type { Resource } from '@/api/grpc'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import TAlert from '@/components/TAlert.vue'
import CloseButton from '@/views/Modals/CloseButton.vue'

const { id, item, search, sensitive } = defineProps<{
  id?: string
  item?: Resource
  search: string
  /** Hide the resource until explicitly revealed, as it may hold secrets. */
  sensitive?: boolean
}>()

defineEmits<{
  close: []
}>()

const revealed = ref(false)

// Every resource needs revealing on its own
watch(
  () => id,
  () => (revealed.value = false),
)

const yaml = computed(() => (item ? dump(item, { noRefs: true }) : ''))
</script>

<template>
  <div class="flex flex-col gap-2 border-l-naturals-n4 bg-naturals-n0 p-4 md:border-l">
    <div class="flex justify-between gap-2">
      <h2 class="truncate font-medium text-naturals-n14">{{ id }}</h2>

      <CloseButton class="shrink-0" @click="$emit('close')" />
    </div>

    <TAlert v-if="!item" type="warn" title="Resource not found">
      This resource no longer exists.
    </TAlert>

    <TAlert
      v-else-if="sensitive && !revealed"
      type="warn"
      title="Sensitive resource"
      :dismiss="{ name: 'Reveal', action: () => (revealed = true) }"
    >
      This resource type may contain secrets such as private keys or tokens.
    </TAlert>

    <!-- pr-1 to give some padding between bg & scrollbar -->
    <div v-else class="min-h-0 overflow-auto pr-1">
      <CodeBlock :code="yaml" lang="yaml" :search />
    </div>
  </div>
</template>
