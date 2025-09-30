<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  label?: string
  disabled?: boolean
  indeterminate?: boolean
}

const { label = '' } = defineProps<Props>()

const checked = defineModel<boolean>({ default: false })
</script>

<template>
  <label class="flex cursor-pointer items-center gap-2 has-disabled:cursor-not-allowed">
    <input v-model="checked" type="checkbox" :disabled="disabled" class="peer sr-only" />

    <div
      class="flex size-3.5 items-center justify-center rounded-xs border border-naturals-n7 peer-checked:border-primary-p6 peer-checked:bg-primary-p6 peer-disabled:border-naturals-n5 peer-disabled:bg-naturals-n4"
    >
      <TAnimation>
        <TIcon
          v-show="checked"
          class="size-full fill-current text-primary-p3"
          :icon="indeterminate ? 'minus' : 'check'"
        />
      </TAnimation>
    </div>

    <span v-if="label" class="block flex-1 truncate text-xs text-naturals-n11 select-none">
      {{ label }}
    </span>
  </label>
</template>
