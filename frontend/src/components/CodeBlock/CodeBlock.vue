<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import type { ComponentProps } from 'vue-component-type-helpers'

import CopyButton from '@/components/CopyButton/CopyButton.vue'
import { type CodeLanguage, highlight } from '@/lib/highlight'

interface Props {
  code?: string
  /** Grammar used for highlighting. */
  lang?: CodeLanguage
  /** Occurrences of this string are tinted on top of the syntax colours. */
  search?: string
  buttonAttrs?: /* @vue-ignore */ Omit<ComponentProps<typeof CopyButton>, 'text'>
}

const { code = '', lang = 'shellscript', search = '', buttonAttrs } = defineProps<Props>()

const highlighted = computedAsync(() => (code ? highlight(code, lang, search) : undefined))
</script>

<template>
  <div class="relative rounded border border-naturals-n7 bg-naturals-n2 text-naturals-n14">
    <div
      class="absolute top-2 right-2 z-10 flex items-center justify-center rounded-md p-1 backdrop-blur"
    >
      <CopyButton v-bind="buttonAttrs" :text="code" />
    </div>

    <div class="p-1">
      <!-- eslint-disable vue/no-v-html -- Shiki escapes the code it highlights. -->
      <pre
        class="overflow-auto px-3 py-1 font-mono text-xs/relaxed whitespace-pre"
      ><span v-if="highlighted" v-html="highlighted"></span><template v-else>{{ code }}</template></pre>
    </div>
  </div>
</template>

<style scoped>
/* v-html output is outside the scope attribute's reach, hence :deep(). */
:deep(.code-search-match) {
  border-radius: var(--radius-xs);

  /* Blue: the syntax theme leaves it free, and it keeps token colours legible
     where a solid fill would not. */
  background-color: color-mix(in srgb, var(--color-blue-b1) 40%, transparent);
}
</style>
