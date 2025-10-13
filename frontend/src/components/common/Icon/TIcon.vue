<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { icons } from './icons'

export type IconType = keyof typeof icons

type Props = {
  svgBase64?: string
  icon?: IconType
}

const { svgBase64 = '', icon = 'action-horizontal' } = defineProps<Props>()

const component = computed(() => icons[icon])
</script>

<template>
  <img v-if="svgBase64" alt="" :src="`data:image/svg+xml;base64,${svgBase64}`" />
  <component :is="component" v-else class="min-w-content" />
</template>
