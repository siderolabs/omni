<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

const {
  image,
  digest,
  search = '',
} = defineProps<{
  image?: string
  digest?: string
  /** Highlights where the image matches the search. */
  search?: string
}>()

const tooltip = computed(() =>
  [image, digest !== image ? digest : undefined].filter(Boolean).join('\n'),
)
</script>

<template>
  <WordHighlighter
    v-if="image"
    :query="search.trim()"
    :text-to-highlight="image"
    highlight-class="search-match"
    class="block font-mono wrap-anywhere text-content-secondary"
    :title="tooltip"
  />
  <span v-else>—</span>
</template>
