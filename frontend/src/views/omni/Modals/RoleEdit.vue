<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate">
        Edit {{ object }} {{ id }}
      </h3>
      <close-button @click="close" />
    </div>

    <div class="flex gap-4 flex-wrap">
      <watch
        :opts="{ resource: { type: UserType, namespace: DefaultNamespace, id: $route.query.user as string }, runtime: Runtime.Omni }"
        class="flex-1">
        <template #default="{ items }">
          <t-select-list v-if="items[0]?.spec?.role" class="h-full" title="Role" :values="roles" :defaultValue="items[0]?.spec?.role"
            @checkedValue="value => role = value" />
        </template>
      </watch>
      <t-button type="highlighted" @click="() => { handleRoleUpdate(); close() }" :disabled="!canManageUsers"
        class="h-9">Update {{ object }}</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref } from "vue";
import { updateRole } from "@/methods/user";
import { showError, showSuccess } from "@/notification";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { RoleNone, RoleReader, RoleOperator, RoleAdmin, DefaultNamespace, UserType } from "@/api/resources";
import { canManageUsers } from "@/methods/auth";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import Watch from "@/components/common/Watch/Watch.vue";

const router = useRouter();
const route = useRoute();


const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string | undefined> = ref();

const object = route.query.serviceAccount ? "Service Account" : "User";
const id = route.query.identity ?? route.query.serviceAccount;

const handleRoleUpdate = async () => {
  if (!role.value) {
    return;
  }

  if (!route.query.user) {
    showError(`Failed to Update ${object}`, "User id is not defined");

    return;
  }

  try {
    await updateRole(route.query.user as string, role.value);
  } catch (e) {
    showError(`Failed to Update ${object}`, e.message)

    return;
  }

  showSuccess(`Successfully Updated ${object} ${id}`)
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
