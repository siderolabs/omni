<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref, toRefs } from 'vue'

import TAnimation from '@/components/Animation/TAnimation.vue'

const props = defineProps<{
  text: string
}>()

const { text } = toRefs(props)
const { copy, copied } = useClipboard({ copiedDuring: 400 })

const showCopyButton = ref(false)
</script>

<template>
  <code
    class="relative overflow-hidden p-tight"
    @mouseenter="() => (showCopyButton = true)"
    @mouseleave="() => (showCopyButton = false)"
  >
    <TAnimation>
      <div
        v-if="showCopyButton"
        class="absolute top-0 right-0 left-0 flex h-14 justify-end rounded bg-linear-to-b from-surface-page p-micro"
      >
        <span class="rounded">
          <button @click="copy(text)">{{ copied ? 'Copied' : 'Copy' }}</button>
        </span>
      </div>
    </TAnimation>
    {{ text }}
  </code>
</template>

<style scoped>
@reference "../../index.css";

code {
  @apply relative rounded bg-surface-hover p-tight break-all whitespace-pre-line;
}

button {
  @apply rounded border border-border-strong bg-surface-hover px-micro py-0.5 transition-colors duration-200 hover:border-border-strong hover:bg-surface-inert hover:text-content-default;
}
</style>
