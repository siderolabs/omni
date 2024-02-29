<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-actions-box style="height: 24px">
    <t-actions-box-item
      icon="log"
      @click.stop="$router.push({ name: 'MachineLogs', params: { machine: clusterMachineStatus.metadata.id! } })"
    >
      Logs
    </t-actions-box-item>
    <t-actions-box-item
      icon="power"
      @click="shutdownNode"
    >Shutdown</t-actions-box-item>
    <t-actions-box-item
      icon="reboot"
      @click="rebootNode"
    >Reboot</t-actions-box-item>
    <t-actions-box-item
      v-if="clusterMachineStatus.spec.stage === ClusterMachineStatusSpecStage.BEFORE_DESTROY"
      icon="rollback"
      @click.stop="restoreNode"
    >
      Cancel Destroy
    </t-actions-box-item>
    <t-actions-box-item
      v-else-if="!deleteDisabled"
      icon="delete"
      danger
      @click.stop="deleteNode"
    >
      Destroy
    </t-actions-box-item>
  </t-actions-box>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { Resource } from "@/api/grpc";
import { ClusterMachineStatusSpec, ClusterMachineStatusSpecStage } from "@/api/omni/specs/omni.pb";

import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";

const router = useRouter();
const props = defineProps<{
  clusterName: string,
  deleteDisabled?: boolean,
  clusterMachineStatus: Resource<ClusterMachineStatusSpec>
}>();

const deleteNode = () => {
  router.push({
    query: {
      modal: "nodeDestroy",
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: "true",
    },
  });
};

const shutdownNode = () => {
  router.push({
    query: {
      modal: "shutdown",
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: "true",
    },
  });
};

const rebootNode = () => {
  router.push({
    query: {
      modal: "reboot",
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: "true",
    },
  });
};

const restoreNode = () => {
  router.push({
    query: {
      modal: "nodeDestroyCancel",
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: "true",
    },
  });
};
</script>