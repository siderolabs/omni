<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  PaginationEllipsis,
  PaginationFirst,
  PaginationLast,
  PaginationList,
  PaginationListItem,
  PaginationNext,
  PaginationPrev,
  PaginationRoot,
} from 'reka-ui'

import IconButton from '@/components/Button/IconButton.vue'
import { cn } from '@/methods/utils'

const { pageCount } = defineProps<{
  pageCount: number
}>()

const currentPage = defineModel<number>('current-page', { default: 1 })
</script>

<template>
  <PaginationRoot
    v-if="pageCount > 1"
    v-model:page="currentPage"
    :sibling-count="1"
    :total="pageCount"
    :items-per-page="1"
    show-edges
  >
    <PaginationList
      v-slot="{ items }"
      :class="cn('flex items-center justify-end gap-4', $attrs.class)"
    >
      <PaginationFirst as-child>
        <IconButton icon="chevron-double-left" />
      </PaginationFirst>

      <PaginationPrev as-child>
        <IconButton icon="chevron-left" />
      </PaginationPrev>

      <template v-for="(page, index) in items">
        <PaginationListItem
          v-if="page.type === 'page'"
          :key="`page-${index}`"
          :value="page.value"
          as-child
        >
          <IconButton class="min-w-6 data-selected:bg-naturals-n4 data-selected:text-naturals-n12">
            {{ page.value }}
          </IconButton>
        </PaginationListItem>

        <PaginationEllipsis v-else :key="`ellipsis-${index}`" class="min-w-6 text-naturals-n11">
          &#8230;
        </PaginationEllipsis>
      </template>

      <PaginationNext as-child>
        <IconButton icon="chevron-right" />
      </PaginationNext>

      <PaginationLast as-child>
        <IconButton icon="chevron-double-right" />
      </PaginationLast>
    </PaginationList>
  </PaginationRoot>
</template>
