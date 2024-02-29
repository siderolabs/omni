<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list-item>
    <template #default>
      <div class="flex items-center gap-2">
        <div class="users-grid text-naturals-N13 flex-1">
          <div class="font-bold">{{ item.metadata.id }}</div>
          <div class="px-2 py-1 max-w-min text-naturals-N10 rounded bg-naturals-N3">{{ props.item.spec.role ?? "None" }}</div>
          <div class="flex flex-wrap gap-1 col-span-3">
            <div v-for="label in labels" :key="label" class="resource-label text-xs label-light6">{{ label }}</div>
          </div>
        </div>
        <div class="flex justify-between">
          <t-actions-box v-if="canManageUsers" style="height: 24px">
            <t-actions-box-item icon="edit" @click.stop="editUser">Edit User</t-actions-box-item>
            <t-actions-box-item icon="delete" @click.stop="deleteUser" danger>Delete User</t-actions-box-item>
          </t-actions-box>
        </div>
      </div>
    </template>
  </t-list-item>
</template>

<script setup lang="ts">
import { UserSpec, IdentitySpec } from "@/api/omni/specs/auth.pb";
import { ResourceTyped } from "@/api/grpc";

import { useRouter } from "vue-router";

import TListItem from "@/components/common/List/TListItem.vue";
import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";
import { canManageUsers } from "@/methods/auth";
import { computed, toRefs } from "vue";
import { SAMLLabelPrefix } from "@/api/resources";

const props = defineProps<{
  item: ResourceTyped<UserSpec & IdentitySpec>
}>();

const { item } = toRefs(props);

const router = useRouter();

const labels = computed(() => {
  return Object.keys(item?.value?.metadata?.labels || {}).filter(
    l => l.startsWith(SAMLLabelPrefix)
  ).map((l: string) => l.replace(`${SAMLLabelPrefix}`, "")) || [];
});

const deleteUser = () => {
  const query: Record<string, string> = {
    user: props.item.spec.user_id!,
    identity: props.item.metadata.id ?? "",
  };

  router.push({
    query: { modal: "userDestroy", ...query },
  });
};

const editUser = () => {
  const query: Record<string, string> = {
    user: props.item.spec.user_id!,
    identity: props.item.metadata.id ?? "",
  };

  router.push({
    query: { modal: "userEdit", ...query },
  });
};
</script>

<style scoped>
.users-grid {
  @apply grid grid-cols-5 pr-2 items-center;
}

.users-grid>* {
  @apply text-xs truncate;
}

.scope > * {
  @apply bg-naturals-N4 p-0.5 px-1 text-naturals-N10;
}

.scope-action-enabled {
  @apply bg-naturals-N4 p-0.5 px-1 text-green-G1;
}

.scope > *:first-child {
  @apply rounded-l;
}

.scope > *:last-child {
  @apply rounded-r;
}
.label-light6 {
  --label-h: 208;
  --label-s: 70;
  --label-l: 86;
  --lighten-by: 0;
}
</style>
