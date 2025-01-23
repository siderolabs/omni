<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Untaint single node cluster
      </h3>
      <close-button @click="closeModal"/>
    </div>
    <p class="text-xs">Apply patch that will enable scheduling user workloads on this node?</p>

    <div class="flex justify-end gap-2 mt-8">
      <t-button @click="onContinue(false)" class="secondary w-15 h-9">No</t-button>
      <t-button type="highlighted" @click="onContinue(true)" class="w-15 h-9">Yes</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { closeModal } from "@/modal";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";

const props = defineProps<{
  onContinue: (untaint: boolean) => void;
}>();

const onContinue = (untaint: boolean) => {
  closeModal();
  props.onContinue(untaint);
};
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
