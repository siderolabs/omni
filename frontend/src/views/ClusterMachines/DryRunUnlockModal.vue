<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import StatusGlyph from '@/components/Status/StatusGlyph.vue'

import type { DryRunFailure } from './dryRunSummary'

const { clusterId, failures } = defineProps<{
  title: string
  actionLabel: string
  clusterId: string
  failures: DryRunFailure[]
  loading?: boolean
}>()

defineEmits<{ confirm: [] }>()

const open = defineModel<boolean>('open', { default: false })
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title
    :action-label
    :loading
    content-class="max-w-2xl"
    @confirm="$emit('confirm')"
  >
    <template #description>
      <slot></slot>
    </template>

    <ul class="flex flex-col rounded border border-border-default text-xs">
      <li
        v-for="failure in failures"
        :key="failure.id"
        class="flex flex-col gap-tight p-compact not-last:border-b not-last:border-border-default"
        data-testid="dry-run-unlock-failure"
      >
        <div class="flex items-center justify-between gap-base">
          <span class="flex items-center gap-micro text-content-emphasis">
            <StatusGlyph glyph="warning" class="text-status-warning-default" aria-hidden="true" />
            {{ failure.name }}
          </span>
          <RouterLink
            class="list-item-link"
            :to="{
              name: 'NodePendingUpdates',
              params: { cluster: clusterId, machine: failure.id },
            }"
            @click="open = false"
          >
            Pending updates
          </RouterLink>
        </div>
        <p v-if="failure.error" class="font-mono wrap-anywhere whitespace-pre-wrap">
          {{ failure.error }}
        </p>
      </li>
    </ul>
  </ConfirmModal>
</template>
