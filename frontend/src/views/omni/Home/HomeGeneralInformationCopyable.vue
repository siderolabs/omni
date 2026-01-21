<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import CopyButton from '@/components/common/CopyButton/CopyButton.vue'

interface Props {
  title: string
  value?: string
  secret?: boolean
}

const secretVisible = ref(false)
const displayValue = computed(() =>
  props.secret && !secretVisible.value ? props.value.replace(/./g, '•') : props.value,
)

const props = withDefaults(defineProps<Props>(), {
  value: '',
})
</script>

<template>
  <div class="flex flex-col gap-1 text-xs">
    <dt class="text-naturals-n11">{{ title }}</dt>

    <div class="flex items-center justify-between gap-1">
      <dd
        class="grow truncate"
        :class="{ 'font-mono': secret }"
        :title="displayValue"
        @click="secretVisible = !secretVisible"
      >
        {{ displayValue }}
      </dd>

      <CopyButton class="shrink-0" :text="value" />
    </div>
  </div>
</template>
