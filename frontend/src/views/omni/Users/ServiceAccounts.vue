<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <t-button @click="openUserCreate" icon="plus" icon-position="left" type="highlighted" :disabled="!canManageUsers">Create Service Account</t-button>
    </div>
    <t-list :opts="watchOpts" pagination class="flex-1" search
      @items-update="fetchAccounts"
      >
      <template #default="{ items }">
        <div class="users-header">
          <div class="users-grid">
            <div>ID</div>
            <div>Role</div>
            <div>Expiration</div>
          </div>
        </div>
        <service-account-item v-for="item in items" :key="itemID(item)" :item="item" :expiration="expirations[item.metadata.id!]"/>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { DefaultNamespace, UserType, IdentityType, LabelIdentityUserID, LabelIdentityTypeServiceAccount, RoleInfraProvider, InfraProviderServiceAccountDomain, ServiceAccountDomain } from "@/api/resources";
import { IdentitySpec } from "@/api/omni/specs/auth.pb";
import { itemID } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import ServiceAccountItem from "@/views/omni/Users/ServiceAccountItem.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { Resource } from "@/api/grpc";
import { canManageUsers } from "@/methods/auth";
import { Ref, watch } from "vue";
import { ManagementService } from "@/api/omni/management/management.pb";
import { ref } from "vue";
import { relativeISO } from "@/methods/time";

const router = useRouter();
const route = useRoute();

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: IdentityType,
      namespace: DefaultNamespace,
    },
    idFunc: (res: Resource<IdentitySpec>) => `default.${(res?.metadata?.labels || {})[LabelIdentityUserID] ?? ""}`,
    selectors: [`${LabelIdentityTypeServiceAccount}`],
  },
  {
    runtime: Runtime.Omni,
    resource: {
      type: UserType,
      namespace: DefaultNamespace,
    },
  },
];

const expirations: Ref<Record<string, string>> = ref({});

const fetchAccounts = async () => {
  const accounts = await ManagementService.ListServiceAccounts({});

  for (const account of accounts.service_accounts ?? []) {
    if (!account.pgp_public_keys) {
      continue;
    }

    const name = account.role === RoleInfraProvider ?
      `${account.name!.split(":")[1]}@${InfraProviderServiceAccountDomain}` :
      `${account.name}@${ServiceAccountDomain}`;

    account.pgp_public_keys.sort((a, b) => a.expiration! > b.expiration! ? 1 : -1);

    expirations.value[name] = relativeISO(account.pgp_public_keys[account.pgp_public_keys.length - 1].expiration!);
  }
};

fetchAccounts();

watch(() => route.query, (newVal: any, old:any) => {
  if(old?.modal && !newVal.modal) {
    fetchAccounts();
  }
});

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
