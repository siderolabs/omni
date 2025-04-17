<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex gap-1 items-start">
      <page-header title="Machine Join Tokens" class="flex-1"/>
    </div>
    <div class="flex justify-end">
      <t-button @click="openUserCreate" icon="plus" icon-position="left" type="highlighted" :disabled="!canManageUsers">Create Join Token</t-button>
    </div>
    <t-list :opts="watchOpts" pagination class="flex-1" search>
      <template #default="{ items }">
        <div class="tokens-header">
          <div class="tokens-grid">
            <div>Name</div>
            <div>Token</div>
            <div>Status</div>
            <div>Expiration</div>
            <div>Use Count</div>
          </div>
        </div>
        <t-list-item v-for="item in items" :key="itemID(item)">
          <div class="flex gap-2">
            <div class="tokens-grid flex-1">
              <div class="flex gap-2 items-center">
                <span class="truncate">{{ item.spec.name ?? 'initial token' }}</span>
                <div v-if="item.spec.is_default"
                  class="px-2 py-1 rounded bg-primary-P3 bg-opacity-10 text-primary-P3">
                  Default
                </div>
              </div>
              <div class="truncate">
                {{ item.metadata.id }}
              </div>
              <t-status :title="getStatusString(item.spec.state)"/>
              <div v-if="item.spec.expiration_time">
                {{ relativeISO(item.spec.expiration_time) }}
              </div>
              <div v-else>
                Never
              </div>
              <div>
                {{ item.spec.use_count ?? 0 }}
              </div>
            </div>
            <t-actions-box>
              <template v-if="item.spec.state === JoinTokenStatusSpecState.ACTIVE">
                <t-actions-box-item icon="copy" @click="() => copyValue(item.metadata.id!)">
                  Copy Token
                </t-actions-box-item>
                <t-actions-box-item icon="copy" @click="() => copyKernelParams(item.metadata.id!)" v-if="connectionParams">
                  Copy Kernel Params
                </t-actions-box-item>
                <t-actions-box-item icon="long-arrow-down" v-if="connectionParams" @click="() => getMachineJoinConfig(item.metadata.id!)">
                  Download Machine Join Config
                </t-actions-box-item>
                <t-actions-box-item icon="long-arrow-down" @click="() => openDownloadInstallationMedia(item.metadata.id!)">
                  Download Installation Media
                </t-actions-box-item>
                <div class="my-0.5 w-full border-naturals-N5 border-b"/>
                <t-actions-box-item icon="check" v-if="!item.spec.is_default" @click="() => makeDefault(item.metadata.id!)">
                  Make Default
                </t-actions-box-item>

                <t-actions-box-item icon="error" danger @click="() => openRevokeToken(item.metadata.id!)">
                  Revoke
                </t-actions-box-item>
              </template>
              <template v-else>
                <t-actions-box-item icon="reset" @click="unrevokeJoinToken(item.metadata.id!)">
                  Unrevoke
                </t-actions-box-item>
                <t-actions-box-item icon="delete" @click="() => openDeleteToken(item.metadata.id!)" danger>
                  Delete
                </t-actions-box-item>
              </template>
            </t-actions-box>
          </div>
        </t-list-item>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { ConfigID, ConnectionParamsType, DefaultNamespace, JoinTokenStatusType, DefaultJoinTokenID, DefaultJoinTokenType } from "@/api/resources";
import { itemID } from "@/api/watch";
import { copyText } from "vue3-clipboard";

import TList from "@/components/common/List/TList.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TListItem from "@/components/common/List/TListItem.vue";
import TStatus from "@/components/common/Status/TStatus.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";

import { canManageUsers, unrevokeJoinToken } from "@/methods/auth";
import { TCommonStatuses } from "@/constants";
import { DefaultJoinTokenSpec, JoinTokenStatusSpecState } from "@/api/omni/specs/auth.pb";
import { relativeISO } from "@/methods/time";
import { onMounted, ref } from "vue";
import { ResourceService } from "@/api/grpc";
import { withRuntime } from "@/api/options";
import { Resource } from "@/api/grpc";
import { ConnectionParamsSpec } from "@/api/omni/specs/siderolink.pb";
import { copyKernelArgs, downloadMachineJoinConfig, parseKernelArgs } from "@/methods";
import { showError } from "@/notification";

const router = useRouter();

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: JoinTokenStatusType,
      namespace: DefaultNamespace,
    },
  },
];

const getStatusString = (state: JoinTokenStatusSpecState): TCommonStatuses => {
  switch (state) {
    case JoinTokenStatusSpecState.ACTIVE:
      return TCommonStatuses.ACTIVE;
    case JoinTokenStatusSpecState.EXPIRED:
      return TCommonStatuses.EXPIRED;
    case JoinTokenStatusSpecState.REVOKED:
      return TCommonStatuses.REVOKED;
  }

  return TCommonStatuses.UNKNOWN;
}

const openUserCreate = () => {
  router.push({
    query: { modal: "joinTokenCreate" },
  });
};

const copyValue = (value: string) => {
  return copyText(value, undefined, () => { });
};

const copyKernelParams = (token: string) => {
  if (!connectionParams.value?.spec.args) {
    return;
  }

  copyKernelArgs(parseKernelArgs(connectionParams.value?.spec.args, token));
};

const makeDefault = async (token: string) => {
  const defaultJoinToken: Resource<DefaultJoinTokenSpec> = await ResourceService.Get({
    namespace: DefaultNamespace,
    id: DefaultJoinTokenID,
    type: DefaultJoinTokenType,
  }, withRuntime(Runtime.Omni));

  defaultJoinToken.spec.token_id = token;

  try {
    await ResourceService.Update(defaultJoinToken, defaultJoinToken.metadata.version, withRuntime(Runtime.Omni));
  } catch (e) {
    showError("Failed to Update Default Join Token", e.message)
  }
};

const getMachineJoinConfig = (token: string) => {
  if (!connectionParams.value?.spec.args) {
    return;
  }

  downloadMachineJoinConfig(parseKernelArgs(connectionParams.value?.spec.args, token));
};

const connectionParams = ref<Resource<ConnectionParamsSpec>>();

onMounted(async () => {
  connectionParams.value = await ResourceService.Get({
    namespace: DefaultNamespace,
    type: ConnectionParamsType,
    id: ConfigID,
  }, withRuntime(Runtime.Omni));
});

const openDownloadInstallationMedia = (token: string) => {
  router.push({
    query: { modal: "downloadInstallationMedia", joinToken: token },
  });
};

const openRevokeToken = (token: string) => {
  router.push({
    query: { modal: "joinTokenRevoke", token: token },
  });
};

const openDeleteToken = (token: string) => {
  router.push({
    query: { modal: "joinTokenDelete", token: token },
  });
};
</script>

<style scoped>
.tokens-grid {
  @apply grid grid-cols-5 pr-10 gap-4 items-center;
}

.tokens-header {
  @apply bg-naturals-N2 mb-1 px-3 py-2 pr-12;
}

.tokens-header > * {
  @apply text-xs;
}
</style>
