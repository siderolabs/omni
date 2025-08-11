<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

defineProps<{
  action: string
  controlPlanes?: number
  workers?: number
  onSubmit: () => void
  onReset?: () => void
  warning?: string
}>()
</script>

<template>
  <TAnimation>
    <div class="menu flex items-center">
      <div class="flex flex-col">
        <div class="menu__amount-box flex-1">
          <span class="menu__amount-box--light">{{ controlPlanes }}</span>
          <span class="menu__amount-box--light"
            >Control {{ pluralize('Plane', controlPlanes, false) }},</span
          >
          <span class="menu__amount-box--light">{{ workers }}</span>
          <span class="menu__amount-box--light">{{ pluralize('Worker', workers, false) }}</span>
          selected
        </div>
        <div v-if="warning" class="text-red-r1">{{ warning }}</div>
      </div>
      <TButton icon-position="left" @click="onSubmit">
        {{ action }}
      </TButton>
      <TIcon class="menu__exit-button" icon="close" @click="onReset" />
    </div>
  </TAnimation>
</template>

<style scoped>
@reference "../../../index.css";

.menu {
  @apply fixed z-20 flex w-full gap-4 rounded bg-naturals-n3 p-5;
  width: 452px;
  min-height: 56px;
  bottom: 32px;
  left: calc(50% - 240px);
}
.menu__amount-box {
  @apply flex items-center text-xs text-naturals-n8;
}
.menu__amount-box--light {
  @apply mr-1 text-naturals-n13;
}
.menu__buttons-box {
  @apply flex items-center;
}
.menu__exit-button {
  @apply h-6 w-6 cursor-pointer fill-current text-naturals-n7 transition-colors hover:text-naturals-n8;
}
</style>
