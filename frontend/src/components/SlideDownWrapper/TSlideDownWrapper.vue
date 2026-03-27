<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useElementSize } from '@vueuse/core'
import { useTemplateRef } from 'vue'

defineProps<{ expanded?: boolean }>()

const bodyRef = useTemplateRef('body')
const { height } = useElementSize(bodyRef)
</script>

<template>
  <div
    class="overflow-hidden transition-all duration-300"
    :style="{ height: expanded ? `${height}px` : '0' }"
  >
    <div ref="body">
      <slot></slot>
    </div>
  </div>
</template>
