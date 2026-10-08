<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
const decoder = new TextDecoder()
</script>

<script setup lang="ts">
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import { formatISO } from '@/methods/time'
import type { AuditLogEvent } from '@/pages/(authenticated)/settings/audit-logs.vue'

const { data } = defineProps<{ data: Uint8Array<ArrayBuffer>; search: string }>()

const open = defineModel<boolean>({ default: false })

const item = computed(() => JSON.parse(decoder.decode(data)))

function getMarkerClassForEvent(event: AuditLogEvent) {
  switch (event) {
    case 'create':
      return 'bg-series-1'
    case 'update_with_conflicts':
    case 'update':
      return 'bg-series-2'
    case 'destroy':
    case 'teardown':
      return 'bg-series-3'
    case 'k8s_access':
      return 'bg-series-4'
    case 'talos_access':
      return 'bg-series-5'
    case 'audit_log_access':
      return 'bg-series-6'
    default:
      const unhandled: never = event

      return unhandled
  }
}

function toggleRow() {
  open.value = !open.value
}
</script>

<template>
  <CollapsibleRoot v-model:open="open" class="group/root contents">
    <CollapsibleTrigger
      as="div"
      role="row"
      tabindex="0"
      class="group/trigger col-span-full grid cursor-pointer grid-cols-subgrid items-center px-tight py-2.5 select-none group-hover/root:bg-surface-hover"
      @keydown.enter.prevent="toggleRow"
      @keydown.space.prevent="toggleRow"
    >
      <div role="cell" aria-hidden="true">
        <div class="size-5 rounded-md bg-surface-inert p-0.5 text-content-muted">
          <TIcon
            icon="dropdown"
            class="size-4 transition-transform group-data-[state=open]/trigger:rotate-180"
          />
        </div>
      </div>

      <div role="cell" class="whitespace-nowrap text-content-default">
        <time :datetime="new Date(item.event_ts).toISOString()">
          {{ formatISO(new Date(item.event_ts).toISOString()) }}
        </time>
      </div>

      <div role="cell">
        <span
          v-if="item.event_type.toUpperCase()"
          class="resource-label inline-flex items-center gap-1.5"
        >
          <span
            aria-hidden="true"
            class="size-2 shrink-0 rounded-full"
            :class="getMarkerClassForEvent(item.event_type)"
          />
          <WordHighlighter
            :query="search"
            :text-to-highlight="item.event_type.toUpperCase()"
            highlight-class="search-match"
          />
        </span>
      </div>

      <div role="cell" class="truncate text-content-muted">
        <WordHighlighter
          :query="search"
          :text-to-highlight="item.resource_type"
          highlight-class="search-match"
          class="text-content-emphasis"
        />
      </div>

      <div role="cell" class="min-w-20 space-x-tight truncate whitespace-nowrap">
        <WordHighlighter
          v-if="item.event_data.session.role"
          :query="search"
          :text-to-highlight="item.event_data.session.role"
          highlight-class="search-match"
          class="text-content-emphasis"
        />

        <span v-else class="text-content-emphasis">System / Service Account</span>

        <WordHighlighter
          :query="search"
          :text-to-highlight="
            item.event_data.session.email
              ? item.event_data.session.email
              : item.event_data.session.user_agent
          "
          highlight-class="search-match"
          class="text-content-muted"
        />
      </div>
    </CollapsibleTrigger>

    <CollapsibleContent
      role="row"
      class="collapsible-content col-span-full overflow-hidden group-hover/root:bg-surface-hover"
    >
      <div role="cell" class="px-tight pb-tight">
        <CodeBlock :code="JSON.stringify(item, null, 2)" lang="json" :search />
      </div>
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
