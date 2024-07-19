<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col w-full gap-4">
    <page-header title="All Nodes"/>
    <t-list :opts="opts" search pagination>
      <template #default="{ items, searchQuery }">
        <div class="nodes-list">
          <div class="nodes-list-heading">
            <p>Name</p>
            <p>IP</p>
            <p>OS</p>
            <p>Roles</p>
            <p>Status</p>
          </div>
          <t-group-animation>
            <nodes-item
              v-for="item in items"
              :key="item.metadata.id!"
              :item="item"
              :searchOption="searchQuery"
            />
          </t-group-animation>
        </div>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { getContext } from "@/context";
import { kubernetes, ClusterMachineStatusType, LabelCluster, ClusterMachineStatusLabelNodeName, TalosMemberType, TalosClusterNamespace } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { ClusterMachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { useRoute } from "vue-router";
import { DefaultNamespace } from "@/api/resources";
import { V1Node } from "@kubernetes/client-node";

import TList from "@/components/common/List/TList.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import NodesItem from "@/views/cluster/Nodes/components/NodesItem.vue";
import TGroupAnimation from "@/components/common/Animation/TGroupAnimation.vue";
import { Resource } from "@/api/grpc";

const route = useRoute();
const context = getContext();

const opts = [
{
  runtime: Runtime.Omni,
  resource: {
    type: ClusterMachineStatusType,
    namespace: DefaultNamespace,
  },
  selectors: [
    `${LabelCluster}=${route.params.cluster}`
  ],
  idFunc: (item: Resource<ClusterMachineStatusSpec>): string => {
    return (item?.metadata?.labels ?? {})[ClusterMachineStatusLabelNodeName] ?? item.metadata.id;
  }
},
{
  runtime: Runtime.Kubernetes,
  resource: {
    type: kubernetes.node,
  },
  context,
  idFunc: (item: V1Node): string => {
    return item.metadata!.name!;
  }
},
{
  runtime: Runtime.Talos,
  resource: {
    type: TalosMemberType,
    namespace: TalosClusterNamespace,
  },
  context,
  idFunc: (item: any): string => {
    return item.metadata.id;
  }
}];
</script>

<style scoped>
.nodes-list-heading {
  @apply flex items-center bg-naturals-N2;
  padding: 10px 16px;
}
.nodes-list-heading > p {
  @apply text-xs text-naturals-N13 w-1/5;
}
</style>
