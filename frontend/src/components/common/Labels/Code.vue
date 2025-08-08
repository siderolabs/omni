<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <code
    @mouseenter="() => (showCopyButton = true)"
    @mouseleave="() => (showCopyButton = false)"
    class="relative p-2 overflow-hidden"
  >
    <t-animation>
      <div
        v-if="showCopyButton"
        class="absolute top-0 left-0 right-0 h-14 flex justify-end p-1 bg-gradient-to-b from-naturals-N0 bg-opacity-25 rounded"
      >
        <span class="rounded">
          <button @click="copyKey">{{ copyState }}</button>
        </span>
      </div>
    </t-animation>
    {{ text }}
  </code>
</template>

<script setup lang="ts">
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import { ref, toRefs } from 'vue'
import { copyText } from 'vue3-clipboard'

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

<style scoped>
code {
  @apply break-all rounded bg-naturals-N4 relative p-2 whitespace-pre-line;
}

button {
  @apply border bg-naturals-N4 border-naturals-N6 rounded px-1 py-0.5 transition-colors duration-200 hover:bg-naturals-N6 hover:border-naturals-N8 hover:text-naturals-N13;
}
</style>
