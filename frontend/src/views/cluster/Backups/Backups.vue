<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="flex gap-1 items-start">
      <page-header title="Control Plane Backups" class="flex-1"/>
      <div class="flex gap-1 items-center text-xs">
        Cluster UUID:
        <div>{{ clusterUUID }}</div>
        <icon-button icon="copy" @click="() => copyText(clusterUUID, undefined, () => {})"/>
        <t-button type="highlighted" class="ml-2" @click="runEtcdBackup" :disabled="startingEtcdBackup || !backupStatus.enabled">Trigger Etcd Backup</t-button>
      </div>
    </div>
    <backups-list class="mb-6"/>
  </div>
</template>

<script setup lang="ts">
import PageHeader from "@/components/common/PageHeader.vue";
import { triggerEtcdBackup } from "@/methods/cluster";
import { showError } from "@/notification";
import { onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { Resource, ResourceService } from "@/api/grpc";
import { withRuntime } from "@/api/options";
import { Runtime } from "@/api/common/omni.pb";
import { ClusterUUIDType, DefaultNamespace } from "@/api/resources";
import { ClusterUUID } from "@/api/omni/specs/omni.pb";
import { copyText } from "vue3-clipboard";

import BackupsList from "@/views/cluster/Backups/BackupsList.vue";
import TButton from "@/components/common/Button/TButton.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import { setupBackupStatus } from "@/methods";

const startingEtcdBackup = ref(false);
const route = useRoute();
const clusterUUID = ref("loading...");
const { status: backupStatus } = setupBackupStatus();

onMounted(async () => {
  const resp: Resource<ClusterUUID> = await ResourceService.Get({
    id: route.params.cluster as string,
    namespace: DefaultNamespace,
    type: ClusterUUIDType,
  }, withRuntime(Runtime.Omni))

  clusterUUID.value = resp.spec.uuid!;
});

const runEtcdBackup = async () => {
  startingEtcdBackup.value = true;

  try {
    await triggerEtcdBackup(route.params.cluster as string);
  } catch (e) {
    showError("Failed to Trigger Manual Etcd Backup", e.message);
  }

  startingEtcdBackup.value = false;
}
</script>
