<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div v-if="!(hideZero && count == 0)" class="flex gap-2 items-center">
    <t-icon class="w-5 h-5" :icon="icon" :style="{ color }"/>
    <div class="text-md text-naturals-N10 whitespace-nowrap">
      <span class="text-naturals-N13">{{ count }}{{ units }}</span> {{ pluralizedText ? pluralize(pluralizedText, count, false) : '' }}{{ text }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconType } from '../Icon/TIcon.vue';
import pluralize from 'pluralize';
import TIcon from '../Icon/TIcon.vue';
import { primary } from '@/vars/colors';

withDefaults(defineProps<{
  icon: IconType
  count: number
  units?: string
  text?: string
  pluralizedText?: string
  color?: string
  hideZero?: boolean
}>(),
  {
    color: primary.P2
  }
)
</script>
