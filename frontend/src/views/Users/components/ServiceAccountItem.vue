<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { isPast } from 'date-fns'
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
import { useId } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { ServiceAccountStatusSpec } from '@/api/omni/specs/auth.pb'
import { RoleInfraProvider } from '@/api/resources'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import { usePermissions } from '@/methods/auth'
import { relativeISO } from '@/methods/time'
import ServiceAccountKeys from '@/views/Users/components/ServiceAccountKeys.vue'

const { item, searchQuery = '' } = defineProps<{
  item: Resource<ServiceAccountStatusSpec>
  lastActive: string
  searchQuery?: string
}>()

const emit = defineEmits<{
  rotate: []
  edit: []
  delete: []
  revokeKey: [publicKeyId: string]
}>()

const open = defineModel<boolean>({ default: false })

const { canManageUsers } = usePermissions()

const regionId = useId()
const labelId = useId()
</script>

<template>
  <CollapsibleRoot
    v-model:open="open"
    as="li"
    class="col-span-full grid grid-cols-subgrid overflow-hidden rounded border border-border-strong text-xs"
    :aria-labelledby="labelId"
  >
    <CollapsibleTrigger
      :aria-labelledby="labelId"
      class="group/collapsible-trigger col-span-full grid grid-cols-subgrid items-center bg-surface-chrome p-compact pl-tight text-left hover:bg-surface-raised"
    >
      <div class="flex min-w-0 items-center gap-tight">
        <TIcon
          class="size-5 shrink-0 rounded-md bg-surface-hover transition-transform duration-250 group-data-[state=open]/collapsible-trigger:rotate-180 hover:text-content-default"
          icon="drop-up"
          aria-hidden="true"
        />

        <span :id="labelId" class="truncate font-bold">
          <WordHighlighter
            :query="searchQuery"
            :text-to-highlight="item.metadata.id"
            split-by-space
            highlight-class="search-match"
          />
        </span>
      </div>

      <div>
        <span class="resource-label">{{ item.spec.role ?? 'None' }}</span>
      </div>

      <div class="text-content-muted">{{ lastActive }}</div>

      <div class="text-content-muted">{{ item.spec.public_keys?.length ?? 0 }}</div>

      <div>
        <template v-if="!item.spec.expiration">-</template>
        <StatusPill v-else-if="isPast(item.spec.expiration)" tone="danger">Expired</StatusPill>
        <template v-else>{{ relativeISO(item.spec.expiration) }}</template>
      </div>

      <div class="flex items-center justify-self-end">
        <TActionsBox v-if="canManageUsers" aria-label="service account actions">
          <TActionsBoxItem icon="arrow-path" @select="emit('rotate')">Renew Key</TActionsBoxItem>

          <TActionsBoxItem
            v-if="item.spec.role !== RoleInfraProvider"
            icon="edit"
            @select="emit('edit')"
          >
            Edit Service Account
          </TActionsBoxItem>

          <TActionsBoxItem icon="delete" danger aria-haspopup="dialog" @select="emit('delete')">
            Delete Service Account
          </TActionsBoxItem>
        </TActionsBox>
      </div>
    </CollapsibleTrigger>

    <CollapsibleContent
      :id="regionId"
      as="section"
      :aria-labelledby="labelId"
      class="collapsible-content col-span-full grid grid-cols-subgrid overflow-hidden"
    >
      <ServiceAccountKeys
        :keys="item.spec.public_keys"
        :can-revoke="canManageUsers"
        @revoke="(publicKeyId) => emit('revokeKey', publicKeyId)"
      />
    </CollapsibleContent>
  </CollapsibleRoot>
</template>

<style scoped>
.collapsible-content[data-state='open'] {
  animation: slideDown 200ms ease-out;
}

.collapsible-content[data-state='closed'] {
  animation: slideUp 200ms ease-out;
}

@keyframes slideDown {
  from {
    height: 0;
  }
  to {
    height: var(--reka-collapsible-content-height);
  }
}

@keyframes slideUp {
  from {
    height: var(--reka-collapsible-content-height);
  }
  to {
    height: 0;
  }
}
</style>
