<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import Popper from 'vue3-popper'

import IconButton from '@/components/common/Button/IconButton.vue'

type PlacementType =
  | 'auto'
  | 'auto-start'
  | 'auto-end'
  | 'top'
  | 'top-start'
  | 'top-end'
  | 'bottom'
  | 'bottom-start'
  | 'bottom-end'
  | 'right'
  | 'right-start'
  | 'right-end'
  | 'left'
  | 'left-start'
  | 'left-end'

type Props = {
  placement?: PlacementType
}

withDefaults(defineProps<Props>(), {
  placement: 'left-start',
})

const open = ref(false)
</script>

<template>
  <div v-click-outside="() => (open = false)" class="actions-box">
    <Popper
      offset-distance="10"
      :placement="placement"
      :show="open"
      offset-skid="30"
      class="z-auto! m-0! block! border-0!"
    >
      <template #content>
        <div class="rounded border border-naturals-n4 bg-naturals-n3" @click.stop="open = false">
          <slot />
        </div>
      </template>
      <IconButton icon="action-horizontal" @click="() => (open = !open)" />
    </Popper>
  </div>
</template>
