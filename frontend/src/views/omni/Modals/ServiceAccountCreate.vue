<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Create Service Account
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex flex-col w-full h-full gap-4">
      <t-notification v-if="notification" v-bind="notification.props"/>

      <div class="flex flex-col gap-2" v-if="!key">
        <t-input title="ID" class="flex-1 h-full" placeholder="..." v-model="name"/>
        <t-input title="Expiration Days" type="number" :min="1" class="flex-1 h-full" v-model="expiration"/>
        <t-select-list
            class="h-full"
            title="Role"
            :values="roles"
            :defaultValue="RoleReader"
            @checkedValue="value => role = value"
        />
      </div>
        <t-button type="highlighted" @click="handleCreate" :disabled="!canManageUsers && authType !== AuthType.SAML" class="h-9">Create Service Account</t-button>
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
import { createServiceAccount } from "@/methods/user";
import { Notification, showError, showSuccess } from "@/notification";
import { useRouter } from "vue-router";
import { RoleNone, RoleReader, RoleOperator, RoleAdmin, RoleInfraProvider } from "@/api/resources";

import TAnimation from "@/components/common/Animation/TAnimation.vue";
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import { AuthType, authType } from "@/methods";
import TNotification from "@/components/common/Notification/TNotification.vue";
import { copyText } from "vue3-clipboard";

const notification: Ref<Notification | null> = shallowRef(null);

const name = ref("");
const expiration = ref(365);
const router = useRouter();

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin, RoleInfraProvider]

const role: Ref<string> = ref(RoleReader)

const key = ref<string>();

const showCopyButton = ref(false);
const copyState = ref("Copy");

const handleCreate = async () => {
  if (name.value === "") {
    showError("Failed to Create Service Account", "Name is not defined", notification);

    return;
  }

  try {
    key.value = await createServiceAccount(name.value, role.value, expiration.value);
  } catch (e) {
    showError("Failed to Create Service Account", e.message, notification);

    return;
  }

  showSuccess("Service Account Was Created", undefined, notification);
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
