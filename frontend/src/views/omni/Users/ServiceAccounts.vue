<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <t-button @click="openUserCreate" icon="plus" icon-position="left" type="highlighted" :disabled="!canManageUsers">Create Service Account</t-button>
    </div>
    <t-list :opts="watchOpts" pagination class="flex-1" search>
      <template #default="{ items }">
        <div class="users-header">
          <div class="users-grid">
            <div>ID</div>
            <div>Role</div>
            <div>Expiration</div>
          </div>
        </div>
        <service-account-item v-for="item in items" :key="itemID(item)" :item="item" :expiration="getExpiration(item)"/>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { ServiceAccountStatusType, EphemeralNamespace } from "@/api/resources";
import { itemID } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import ServiceAccountItem from "@/views/omni/Users/ServiceAccountItem.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageUsers } from "@/methods/auth";
import { Resource } from "@/api/grpc";
import { ServiceAccountStatusSpec } from "@/api/omni/specs/auth.pb";
import { relativeISO } from "@/methods/time";

const router = useRouter();

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: ServiceAccountStatusType,
      namespace: EphemeralNamespace
    },
  },
];

const getExpiration = (item: Resource<ServiceAccountStatusSpec>) => {
  return relativeISO(item.spec.public_keys?.[(item.spec.public_keys?.length ?? 0) - 1].expiration ?? "");
};

const openUserCreate = () => {
  router.push({
    query: { modal: "serviceAccountCreate" },
  });
};
</script>

<style scoped>
.users-grid {
  @apply grid grid-cols-3 pr-10;
}

.users-header {
  @apply bg-naturals-N2 mb-1;
  padding: 10px 16px;
}

.users-header>* {
  @apply text-xs;
}
</style>
