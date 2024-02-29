<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Create User
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex gap-2 items-center">
      <t-input title="User Email" class="flex-1 h-full" placeholder="..." v-model="identity"/>
      <t-select-list
          class="h-full"
          title="Role"
          :values="roles"
          :defaultValue="RoleReader"
          @checkedValue="value => role = value"
      />
      <t-button type="highlighted" @click="handleUserCreate" :disabled="!canManageUsers && authType !== AuthType.SAML" class="w-32 h-9">Create User</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref } from "vue";
import { createUser } from "@/methods/user";
import { showError, showSuccess } from "@/notification";
import { useRouter } from "vue-router";
import { RoleNone, RoleReader, RoleOperator, RoleAdmin } from "@/api/resources";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import { AuthType, authType } from "@/methods";

const identity = ref("");
const router = useRouter();

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string> = ref(RoleReader);

const handleUserCreate = async () => {
  if (identity.value === "") {
    showError("Failed to Create User", "User email is not defined");

    return;
  }

  try {
    await createUser(identity.value, role.value);
  } catch (e) {
    showError("Failed to Create User", e.message);

    return;
  }

  close();

  showSuccess(
      `The User ${identity.value} is created`,
  );
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
</style>
