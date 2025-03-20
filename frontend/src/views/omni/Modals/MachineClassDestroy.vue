<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Destroy the Machine Class {{ $route.query.classname }} ?
      </h3>
      <close-button @click="close"/>
    </div>

    <p class="text-xs py-2">Please confirm the action.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="destroy" class="w-32 h-9" icon="delete" iconPosition="left">
        Destroy
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import { ResourceService } from "@/api/grpc";
import { Runtime } from "@/api/common/omni.pb";
import { withRuntime } from "@/api/options";
import { DefaultNamespace, MachineClassType } from "@/api/resources";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";

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

const destroy = async () => {
  try {
    await ResourceService.Delete({
      id: route.query.classname as string,
      namespace: DefaultNamespace,
      type: MachineClassType,
    }, withRuntime(Runtime.Omni));
  } catch (e) {
    showError("Failed to remove the machine class", e.message)

    close();

    return;
  }

  close();

  showSuccess(
    `The Machine Class ${route.query.classname} was Destroyed`,
  );
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
