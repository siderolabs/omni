<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate">
        Renew the Key for the Account {{ $route.query.serviceAccount }}
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex flex-col w-full h-full gap-4">
      <t-notification v-if="notification" v-bind="notification.props"/>

      <div class="flex flex-col gap-2" v-if="!key">
        <t-input title="Expiration Days" type="number" :min="1" class="flex-1 h-full" v-model="expiration"/>
        <t-button type="highlighted" @click="handleRenew" :disabled="!canManageUsers && authType !== AuthType.SAML" class="h-9">Generate New Key</t-button>
      </div>
    </div>

    <ServiceAccountKey v-if="key" :secret-key="key"/>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, shallowRef } from "vue";
import { renewServiceAccount } from "@/methods/user";
import { Notification, showError, showSuccess } from "@/notification";
import { useRoute, useRouter } from "vue-router";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import { AuthType, authType } from "@/methods";
import TNotification from "@/components/common/Notification/TNotification.vue";
import TInput from "@/components/common/TInput/TInput.vue";

import ServiceAccountKey from "./components/ServiceAccountKey.vue";

const notification: Ref<Notification | null> = shallowRef(null);

const router = useRouter();
const route = useRoute();

const key = ref<string>();

const expiration = ref(365);

const handleRenew = async () => {
  try {
    key.value = await renewServiceAccount(route.query.serviceAccount as string, expiration.value);
  } catch (e) {
    showError("Failed to Renew Service Account", e.message, notification);

    return;
  }

  showSuccess("Service Account Key Was Renewed", undefined, notification);
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
