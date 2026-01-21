<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import { closeModal } from '@/modal'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const props = defineProps<{
  onContinue: (untaint: boolean) => void
}>()

const onContinue = (untaint: boolean) => {
  closeModal()
  props.onContinue(untaint)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Untaint single node cluster</h3>
      <CloseButton @click="closeModal" />
    </div>
    <p class="text-xs">Apply patch that will enable scheduling user workloads on this node?</p>

    <div class="mt-8 flex justify-end gap-2">
      <TButton class="secondary h-9 w-15" @click="onContinue(false)">No</TButton>
      <TButton type="highlighted" class="h-9 w-15" @click="onContinue(true)">Yes</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
