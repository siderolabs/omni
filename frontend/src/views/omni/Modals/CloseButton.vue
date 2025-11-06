<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ButtonHTMLAttributes } from 'vue'

import TIcon from '@/components/common/Icon/TIcon.vue'

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
interface Props extends /* @vue-ignore */ ButtonHTMLAttributes {}

defineProps<Props>()
</script>

<template>
  <button
    type="button"
    aria-label="close"
    class="size-6 text-naturals-n10 transition-colors hover:text-naturals-n14 active:text-naturals-n9"
  >
    <TIcon class="size-full" icon="close" />
  </button>
</template>
