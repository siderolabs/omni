<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { copyText } from 'vue3-clipboard'

import TIcon from '@/components/common/Icon/TIcon.vue'

const { text = '' } = defineProps<{ text?: string }>()
</script>

<template>
  <button
    aria-label="copy"
    class="text-primary-p2 hover:text-primary-p1 active:text-primary-p3"
    @click.stop="copyText(text, undefined, () => {})"
  >
    <TIcon icon="copy" class="size-4" />
  </button>
</template>
