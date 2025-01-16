<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Accept the Machine {{ $route.query.machine }} ?
      </h3>
      <close-button @click="close"/>
    </div>

    <p class="text-xs py-2">Please confirm the action.</p>

    <div class="text-xs">
      <p class="text-primary-P3 py-2 font-bold">
        Accepting the machine will wipe ALL of its disks.
      </p>
    </div>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="reject" class="w-32 h-9" icon="check" iconPosition="left">
        Accept
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { acceptMachine } from "@/methods/machine";

const router = useRouter();
const route = useRoute();

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const reject = async () => {
  try {
    await acceptMachine(route.query.machine as string);
  } catch (e) {
    showError(`Failed to Accept the Machine ${route.query.machine}`, e.message)
  }

  close();

  showSuccess(
    `The Machine ${route.query.machine} was Accepted`,
  );
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
