<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Destroy the Machine {{ $route.query.machine }} ?
      </h3>
      <close-button @click="close"/>
    </div>

    <p class="text-xs py-2">Please confirm the action.</p>

    <div v-if="$route.query.cluster" class="text-xs">
      <p class="text-primary-P3 py-2">
        The machine <code>{{ $route.query.machine }}</code> is part of the cluster <code>{{ $route.query.cluster }}</code>.
        Destroying the machine should be only used as a last resort, e.g. in a case of a hardware failure.
      </p>
      <p class="text-primary-P3 py-2 font-bold">
        The machine will need to wiped and reinstalled to be used again with Omni.
      </p>

      <p class="py-2">
        If you want to remove the machine from the cluster, please use the <router-link :to="{ name: 'ClusterOverview', params: { cluster: $route.query.cluster }}">cluster overview page</router-link>.
      </p>
    </div>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="remove" class="w-32 h-9" icon="delete" iconPosition="left" :disabled="disabled">
        Remove
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import { removeMachine } from "@/methods/machine";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";

const router = useRouter();
const route = useRoute();
const disabled = ref(false);

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;
  disabled.value = false;

  router.go(-1);
};

const remove = async () => {
  disabled.value = true;

  try {
    await removeMachine(route.query.machine as string)
  } catch (e) {
    showError("Failed to remove the machine", e.message)
  }

  close();

  showSuccess(
    `The Machine ${route.query.machine} was Removed`,
  );
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
