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

function getLabelClassForEvent(event: AuditLogEvent) {
  switch (event) {
    case 'k8s_access':
      return 'label-orange'
    case 'create':
      return 'label-green'
    case 'update_with_conflicts':
    case 'update':
      return 'label-blue'
    case 'destroy':
    case 'teardown':
      return 'label-red'
    case 'talos_access':
    default:
      return
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
      class="group/trigger col-span-full grid cursor-pointer grid-cols-subgrid items-center px-2 py-2.5 select-none group-hover/root:bg-white/5"
      @keydown.enter.prevent="toggleRow"
      @keydown.space.prevent="toggleRow"
    >
      <div role="cell" aria-hidden="true">
        <div class="size-5 rounded-md bg-naturals-n5 p-0.5 text-naturals-n10">
          <TIcon
            icon="dropdown"
            class="transition-transform group-data-[state=open]/trigger:rotate-180"
          />
        </div>
      </div>

      <div role="cell" class="whitespace-nowrap text-naturals-n13">
        <time :datetime="new Date(item.event_ts).toISOString()">
          {{ formatISO(new Date(item.event_ts).toISOString()) }}
        </time>
      </div>

      <div role="cell">
        <WordHighlighter
          v-if="item.event_type.toUpperCase()"
          :query="search"
          :text-to-highlight="item.event_type.toUpperCase()"
          highlight-class="bg-naturals-n14"
          class="resource-label"
          :class="getLabelClassForEvent(item.event_type)"
        />
      </div>

      <div role="cell" class="truncate text-naturals-n10">
        <WordHighlighter
          :query="search"
          :text-to-highlight="item.resource_type"
          highlight-class="bg-naturals-n14"
          class="text-naturals-n14"
        />
      </div>

      <div role="cell" class="min-w-20 space-x-2 truncate whitespace-nowrap">
        <WordHighlighter
          v-if="item.event_data.session.role"
          :query="search"
          :text-to-highlight="item.event_data.session.role"
          highlight-class="bg-naturals-n14"
          class="text-naturals-n14"
        />

        <span v-else class="text-naturals-n14">System / Service Account</span>

        <WordHighlighter
          :query="search"
          :text-to-highlight="
            item.event_data.session.email
              ? item.event_data.session.email
              : item.event_data.session.user_agent
          "
          highlight-class="bg-naturals-n14"
          class="text-naturals-n10"
        />
      </div>
    </CollapsibleTrigger>

    <CollapsibleContent
      role="row"
      class="collapsible-content col-span-full overflow-hidden group-hover/root:bg-white/5"
    >
      <div role="cell" class="px-2 pb-2">
        <CodeBlock>
          <WordHighlighter
            :query="search"
            :text-to-highlight="JSON.stringify(item, null, 2)"
            highlight-class="bg-naturals-n14"
            class="text-naturals-n10"
          />
        </CodeBlock>
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
