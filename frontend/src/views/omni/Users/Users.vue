<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <t-button @click="openUserCreate" icon="user-add" icon-position="left" type="highlighted" :disabled="!canManageUsers || authType === AuthType.SAML">Add User</t-button>
    </div>
    <t-list :opts="watchOpts" pagination class="flex-1" search>
      <template #default="{ items }">
        <div class="users-header">
          <div class="users-grid">
            <div>Email</div>
            <div>Role</div>
            <div class="col-span-3">Labels</div>
          </div>
        </div>
        <user-item v-for="item in items" :key="itemID(item)" :item="item"/>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { DefaultNamespace, UserType, IdentityType, LabelIdentityUserID, LabelIdentityTypeServiceAccount } from "@/api/resources";
import { IdentitySpec } from "@/api/omni/specs/auth.pb";
import { itemID } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import UserItem from "@/views/omni/Users/UserItem.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { Resource } from "@/api/grpc";
import { canManageUsers } from "@/methods/auth";
import { AuthType, authType } from "@/methods";

const router = useRouter();

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: IdentityType,
      namespace: DefaultNamespace,
    },
    idFunc: (res: Resource<IdentitySpec>) => `default.${(res?.metadata?.labels || {})[LabelIdentityUserID] ?? ""}`,
    selectors: [`!${LabelIdentityTypeServiceAccount}`],
  },
  {
    runtime: Runtime.Omni,
    resource: {
      type: UserType,
      namespace: DefaultNamespace,
    },
  },
];

const openUserCreate = () => {
  router.push({
    query: { modal: "userCreate" },
  });
};
</script>

<style scoped>
.users-grid {
  @apply grid grid-cols-5 pr-2;
}

.users-header {
  @apply bg-naturals-N2 mb-1;
  padding: 10px 16px;
}

.users-header>* {
  @apply text-xs;
}
</style>
