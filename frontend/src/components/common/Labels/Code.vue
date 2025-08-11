<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'
import { copyText } from 'vue3-clipboard'

import TAnimation from '@/components/common/Animation/TAnimation.vue'

const props = defineProps<{
  text: string
}>()

const { text } = toRefs(props)

const showCopyButton = ref(false)
const copyState = ref('Copy')

let timeout: number

const copyKey = () => {
  copyText(text.value, undefined, () => {})

  copyState.value = 'Copied'

  if (timeout !== undefined) {
    clearTimeout(timeout)
  }

  timeout = setTimeout(() => {
    copyState.value = 'Copy'
  }, 400)
}
</script>

<template>
  <code
    class="relative overflow-hidden p-2"
    @mouseenter="() => (showCopyButton = true)"
    @mouseleave="() => (showCopyButton = false)"
  >
    <TAnimation>
      <div
        v-if="showCopyButton"
        class="absolute top-0 right-0 left-0 flex h-14 justify-end rounded bg-linear-to-b from-naturals-n0 p-1"
      >
        <span class="rounded">
          <button @click="copyKey">{{ copyState }}</button>
        </span>
      </div>
    </TAnimation>
    {{ text }}
  </code>
</template>

<style scoped>
@reference "../../../index.css";

code {
  @apply relative rounded bg-naturals-n4 p-2 break-all whitespace-pre-line;
}

button {
  @apply rounded border border-naturals-n6 bg-naturals-n4 px-1 py-0.5 transition-colors duration-200 hover:border-naturals-n8 hover:bg-naturals-n6 hover:text-naturals-n13;
}
</style>
