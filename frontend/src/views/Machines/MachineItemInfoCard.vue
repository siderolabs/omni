<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, type RenderFunction } from 'vue'

import Card from '@/components/Card/Card.vue'

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
  <Card v-if="filteredSections.length > 0">
    <h3 class="px-compact pt-snug pb-tight text-sm font-medium text-content-emphasis">
      {{ title }}
    </h3>

    <dl
      class="flex flex-col items-start gap-micro p-compact text-xs wrap-anywhere text-content-default"
    >
      <template
        v-for="({ title: sectionTitle, value, emptyText }, sectionIndex) in filteredSections"
        :key="sectionIndex"
      >
        <dt class="font-medium text-content-emphasis not-first-of-type:mt-snug">
          {{ sectionTitle }}
        </dt>

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
