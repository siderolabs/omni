<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, type RenderFunction } from 'vue'

import Card from '@/components/common/Card/Card.vue'

const { sections } = defineProps<{
  title: string
  sections: { title: string; value?: string | string[] | RenderFunction; emptyText?: string }[]
}>()

// Hide blank sections with no empty text
const filteredSections = computed(() =>
  sections.filter(({ emptyText, value }) => {
    const hasValue = Array.isArray(value)
      ? value.length
      : typeof value === 'string'
        ? value.trim()
        : value

    return hasValue || emptyText
  }),
)
</script>

<template>
  <Card>
    <h3 class="px-4 pt-3 pb-2 text-sm font-medium text-naturals-n14">{{ title }}</h3>

    <dl class="flex flex-col items-start gap-1 p-4 text-xs wrap-anywhere text-naturals-n12">
      <template
        v-for="({ title: sectionTitle, value, emptyText }, sectionIndex) in filteredSections"
        :key="sectionIndex"
      >
        <dt class="font-medium text-naturals-n14 not-first-of-type:mt-3">{{ sectionTitle }}</dt>

        <template v-if="typeof value === 'function'">
          <component :is="value" />
        </template>

        <template v-else-if="!value?.length">
          <dd>{{ emptyText }}</dd>
        </template>

        <template v-else-if="Array.isArray(value)">
          <dd v-for="(item, itemIndex) in value" :key="itemIndex">{{ item }}</dd>
        </template>

        <template v-else>
          <dd>{{ value }}</dd>
        </template>
      </template>
    </dl>
  </Card>
</template>
