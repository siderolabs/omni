<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Create Join Token
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex flex-col w-full h-full gap-4">
      <t-notification v-if="notification" v-bind="notification.props"/>

      <template v-if="!key">
        <div class="flex flex-col gap-2">
          <t-input title="Name" class="flex-1 h-full" v-model="name" :onClear="() => name = ''"/>
          <div class="flex gap-1 items-center text-xs">
            Lifetime:
            <t-button-group :options="[
                {
                  label: 'No Expiration',
                  value: Lifetime.NeverExpire
                },
                {
                  label: 'Limited',
                  value: Lifetime.Limited
                }
              ]"
              v-model="lifetime"
            />
          </div>
          <t-input :disabled="lifetime === Lifetime.NeverExpire" title="Expiration Days" type="number" :min="1" class="flex-1" v-model="expiration"/>
        </div>
        <t-button type="highlighted" @click="handleCreate" :disabled="!canManageUsers && authType !== AuthType.SAML" class="h-9">Create Join Token</t-button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, shallowRef } from "vue";
import { createJoinToken } from "@/methods/user";
import { Notification, showError, showSuccess } from "@/notification";
import { useRouter } from "vue-router";

import TButtonGroup from "@/components/common/Button/TButtonGroup.vue";
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import TInput from "@/components/common/TInput/TInput.vue";
import { AuthType, authType } from "@/methods";
import TNotification from "@/components/common/Notification/TNotification.vue";

enum Lifetime {
  NeverExpire = "Never Expire",
  Limited = "Limited"
}

const notification: Ref<Notification | null> = shallowRef(null);

const expiration = ref(365);
const router = useRouter();

const lifetime: Ref<string> = ref(Lifetime.NeverExpire);

const key = ref<string>();
const name = ref<string>("");

const handleCreate = async () => {
  if (name.value == "") {
    showError("Failed to Create Join Token", "Name is not defined", notification);

    return;
  }

  try {
    await createJoinToken(name.value, lifetime.value === Lifetime.Limited ? expiration.value : undefined);
  } catch (e) {
    showError("Failed to Create Join Token", e.message, notification);

    return;
  }

  close();

  showSuccess("Join Token Was Created", undefined, notification);
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
