<!--
Copyright (c) 2024 Sidero Labs, Inc.

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

    <div v-if="key" class="flex flex-col text-xs gap-1 mt-5">
      <span class="text-naturals-N13">Service account key:</span>
      <code @mouseenter="() => showCopyButton = true" @mouseleave="() => showCopyButton = false" class="relative p-2">
        <t-animation>
          <div v-if="showCopyButton" class="absolute top-0 left-0 right-0 h-14 flex justify-end p-2 bg-gradient-to-b from-naturals-N0 rounded">
            <span class="rounded">
              <t-button @click="copyKey" type="compact">{{ copyState }}</t-button>
            </span>
          </div>
        </t-animation>
        {{ key }}
      </code>

      <span class="text-primary-P2 font-bold">Store the key securely as it will not be displayed again.</span>
      <span>The service account can now be used through <code class="p-1">OMNI_ENDPOINT</code> and <code class="p-1">OMNI_SERVICE_ACCOUNT_KEY</code> variables in the CLI.</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, shallowRef } from "vue";
import { renewServiceAccount } from "@/methods/user";
import { Notification, showError, showSuccess } from "@/notification";
import { useRoute, useRouter } from "vue-router";

import TAnimation from "@/components/common/Animation/TAnimation.vue";
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import { AuthType, authType } from "@/methods";
import TNotification from "@/components/common/Notification/TNotification.vue";
import { copyText } from "vue3-clipboard";
import TInput from "@/components/common/TInput/TInput.vue";

const notification: Ref<Notification | null> = shallowRef(null);

const router = useRouter();
const route = useRoute();

const key = ref<string>();

const showCopyButton = ref(false);
const copyState = ref("Copy");
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

let timeout: NodeJS.Timeout

const copyKey = () => {
  copyText(key.value, undefined, () => {});

  copyState.value = "Copied"

  if (timeout !== undefined) {
    clearTimeout(timeout);
  }

  timeout = setTimeout(() => {
    copyState.value = "Copy"
  }, 400);
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
