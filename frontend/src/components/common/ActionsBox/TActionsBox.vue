<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div
    class="actions-box"
    v-click-outside="() => open = false"
  >
    <Popper offsetDistance="10" :placement="placement" :show="open" offsetSkid="30" class="popper">
      <template #content>
        <div class="actions-list" @click.stop="open = false">
          <slot/>
        </div>
      </template>
      <icon-button icon="action-horizontal" @click="() => open = !open"/>
    </Popper>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";

import Popper from "vue3-popper";
import IconButton from "@/components/common/Button/IconButton.vue";

type PlacementType = "auto"
  | "auto-start"
  | "auto-end"
  | "top"
  | "top-start"
  | "top-end"
  | "bottom"
  | "bottom-start"
  | "bottom-end"
  | "right"
  | "right-start"
  | "right-end"
  | "left"
  | "left-start"
  | "left-end";

type Props = {
  placement?: PlacementType;
};

withDefaults(defineProps<Props>(), {
  placement: "left-start",
});

const open = ref(false);
</script>

<style scoped>
.actions-list {
  @apply bg-naturals-N3 rounded border border-naturals-N4;
}

.popper {
  margin: 0 !important;
  border: 0 !important;
  display: block !important;
  z-index: auto !important;
  display: block !important;
}
</style>
