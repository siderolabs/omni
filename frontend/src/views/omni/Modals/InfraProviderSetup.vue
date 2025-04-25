<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Setup a new Infra Provider
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex flex-col w-full h-full gap-4">
      <t-notification v-if="notification" v-bind="notification.props"/>

      <template v-if="!key">
        <div class="flex flex-col gap-2">
          <t-input title="Provider ID" class="flex-1 h-full" placeholder="examples: kubevirt, bare-metal" v-model="id"/>
        </div>
        <t-button type="highlighted" @click="handleCreate" class="h-9">Next</t-button>
      </template>
    </div>

    <ServiceAccountKey v-if="key" :secret-key="key"/>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, shallowRef } from "vue";
import { Notification, showError } from "@/notification";
import { useRouter } from "vue-router";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TNotification from "@/components/common/Notification/TNotification.vue";
import ServiceAccountKey from "./components/ServiceAccountKey.vue";
import { setupInfraProvider } from "@/methods/providers";

const notification: Ref<Notification | null> = shallowRef(null);

const id = ref("");
const router = useRouter();

const key = ref<string>();

const handleCreate = async () => {
  if (id.value === "") {
    showError("Failed to Create Service Account", "Name is not defined", notification);

    return;
  }

  try {
    key.value = await setupInfraProvider(id.value);
  } catch (e) {
    showError("Failed to Create Service Account", e.message, notification);

    return;
  }
};

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};
</script>

<style scoped>
.window {
  @apply rounded bg-naturals-N2 z-30 w-1/3 flex flex-col p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

code {
 @apply break-all rounded bg-naturals-N4;
}
</style>
