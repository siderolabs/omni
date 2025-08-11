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
        class="absolute left-0 right-0 top-0 flex h-14 justify-end rounded bg-opacity-25 bg-gradient-to-b from-naturals-N0 p-1"
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
code {
  @apply relative whitespace-pre-line break-all rounded bg-naturals-N4 p-2;
}

button {
  @apply rounded border border-naturals-N6 bg-naturals-N4 px-1 py-0.5 transition-colors duration-200 hover:border-naturals-N8 hover:bg-naturals-N6 hover:text-naturals-N13;
}
</style>
