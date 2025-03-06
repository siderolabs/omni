<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <watch :opts="watchOverallStatusOpts" spinner noRecordsAlert errorsAlert>
    <template #default="overallStatus">
      <t-alert
        v-if="overallStatus.items[0]?.spec?.configuration_error"
        type="warn"
        :title="`The backups storage is not properly configured: ${overallStatus.items[0]?.spec?.configuration_error}`"
      >
        <div class="flex gap-1">
          Check the
          <t-button type="subtle" @click="openDocs">documentation</t-button> on
          how to configure s3 backups using CLI.
        </div>
        <div class="flex gap-1" v-if="canManageBackupStore">
          Or<t-button
            @click="$router.push({ name: 'BackupStorage' })"
            type="subtle"
            >configure backups in the UI.</t-button
          >
        </div>
      </t-alert>
      <watch v-else :opts="watchStatusOpts" spinner noRecordsAlert errorsAlert>
        <template #default="status">
          <t-list
            :opts="watchOpts"
            search
            :sortOptions="sortOptions"
            :key="status.items[0]?.metadata?.updated"
          >
            <template #default="{ items, searchQuery }">
              <div class="header">
                <div class="list-grid">
                  <div>ID</div>
                  <div>Creation Date</div>
                  <div>Size</div>
                  <div>Snapshot ID</div>
                </div>
              </div>
              <t-list-item v-for="item in items" :key="item.metadata.id!">
                <div
                  class="text-naturals-N12 relative pr-3"
                  :class="{ 'pl-7': !item.spec.description }"
                >
                  <div class="list-grid">
                    <WordHighlighter
                      :query="searchQuery"
                      :textToHighlight="item.metadata.id"
                      highlightClass="bg-naturals-N14"
                    />
                    <div class="text-naturals-N14">
                      {{
                        formatISO(item.spec.created_at as string, dateFormat)
                      }}
                    </div>
                    <div class="text-naturals-N14">
                      {{ formatBytes(parseInt(item.spec.size ?? "0")) }}
                    </div>
                    <div class="text-naturals-N14 flex gap-2 items-center">
                      {{ item.spec.snapshot }}
                      <icon-button
                        icon="copy"
                        @click="
                          copyText(item.spec.snapshot, undefined, () => {})
                        "
                      />
                    </div>
                  </div>
                </div>
              </t-list-item>
            </template>
          </t-list>
        </template>
      </watch>
    </template>
  </watch>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import {
  ExternalNamespace,
  EtcdBackupType,
  LabelCluster,
  EtcdBackupStatusType,
  MetricsNamespace,
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
} from "@/api/resources";
import { WatchOptions } from "@/api/watch";
import { computed } from "vue";
import { formatISO } from "@/methods/time";
import { useRoute } from "vue-router";
import { copyText } from "vue3-clipboard";
import { formatBytes } from "@/methods";

import TList from "@/components/common/List/TList.vue";
import TListItem from "@/components/common/List/TListItem.vue";
import TAlert from "@/components/TAlert.vue";
import WordHighlighter from "vue-word-highlighter";
import IconButton from "@/components/common/Button/IconButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import { canManageBackupStore } from "@/methods/auth";

const dateFormat = "HH:mm MMM d y";
const route = useRoute();

const sortOptions = [
  { id: "id", desc: "Creation Time ⬇", descending: true },
  { id: "id", desc: "Creation Time ⬆" },
];

const watchStatusOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: MetricsNamespace,
      type: EtcdBackupStatusType,
      id: route.params.cluster as string,
    },
    runtime: Runtime.Omni,
  };
});

const watchOverallStatusOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: MetricsNamespace,
      type: EtcdBackupOverallStatusType,
      id: EtcdBackupOverallStatusID,
    },
    runtime: Runtime.Omni,
  };
});

const watchOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: ExternalNamespace,
      type: EtcdBackupType,
    },
    runtime: Runtime.Omni,
    selectors: [`${LabelCluster}=${route.params.cluster}`],
  };
});

const openDocs = () => {
  window
    .open(
      "https://omni.siderolabs.com/how-to-guides/etcd-backups#s3-configuration",
      "_blank"
    )
    ?.focus();
};
</script>

<style scoped>
.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs pl-10;
}

.list-grid {
  @apply grid grid-cols-4 items-center justify-center pr-12 gap-1;
}
</style>
