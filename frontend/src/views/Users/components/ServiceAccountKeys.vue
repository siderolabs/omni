<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { isPast } from 'date-fns'
import { computed } from 'vue'

import type { ServiceAccountStatusSpecPgpPublicKey } from '@/api/omni/specs/auth.pb'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { relativeISO } from '@/methods/time'

const { keys = [] } = defineProps<{
  keys?: ServiceAccountStatusSpecPgpPublicKey[]
  canRevoke?: boolean
}>()

const emit = defineEmits<{
  revoke: [publicKeyId: string]
}>()

const sortedKeys = computed(() =>
  keys.toSorted((a, b) => (b.created ?? '').localeCompare(a.created ?? '')),
)
</script>

<template>
  <section
    class="col-span-full grid grid-cols-subgrid border-t-8 border-border-default text-content-emphasis"
    aria-label="Public keys"
  >
    <div
      v-if="sortedKeys.length"
      class="col-span-full grid grid-cols-subgrid border-border-strong bg-surface-chrome py-tight pr-compact pl-tight text-xs text-content-secondary"
    >
      <div class="ml-base">Public Key ID</div>
      <div>Created</div>
      <div>Last Used</div>
      <div></div>
      <div>Expiration</div>
    </div>

    <p
      v-if="!sortedKeys.length"
      class="col-span-full border-t border-border-default p-tight pl-9 text-xs text-content-muted"
    >
      No keys
    </p>

    <div
      v-for="key in sortedKeys"
      :key="key.id"
      class="col-span-full grid grid-cols-subgrid items-center border-t border-border-default p-tight pr-compact text-xs text-content-emphasis hover:bg-surface-raised"
    >
      <div class="ml-base flex min-w-0 items-center gap-tight">
        <TIcon icon="key" class="size-4 shrink-0" aria-hidden="true" />
        <span class="truncate font-mono">{{ key.id }}</span>
      </div>

      <div class="text-content-muted">{{ key.created ? relativeISO(key.created) : '-' }}</div>

      <div class="text-content-muted">
        {{ key.last_used ? relativeISO(key.last_used) : 'Never' }}
      </div>

      <div></div>

      <div>
        <StatusPill v-if="key.expiration && isPast(key.expiration)" tone="danger">
          Expired
        </StatusPill>
        <template v-else>{{ key.expiration ? relativeISO(key.expiration) : '-' }}</template>
      </div>

      <div class="flex items-center justify-end">
        <Tooltip
          v-if="canRevoke"
          :description="
            sortedKeys.length > 1
              ? 'Revoke key'
              : 'The last key cannot be revoked, delete the service account instead'
          "
        >
          <span class="inline-flex">
            <IconButton
              icon="delete"
              class="hover:text-status-danger-text"
              aria-label="Revoke key"
              :disabled="sortedKeys.length <= 1"
              @click="emit('revoke', key.id!)"
            />
          </span>
        </Tooltip>
      </div>
    </div>
  </section>
</template>
