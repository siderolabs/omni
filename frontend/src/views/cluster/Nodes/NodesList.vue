<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col w-full gap-4">
    <page-header title="All Nodes"/>
    <div class="flex gap-2">
      <t-input
        primary
        class="flex-1"
        placeholder="Search..."
        v-model="searchOption"
      />
      <t-select-list
        title="Status"
        :defaultValue="NodesViewFilterOptions.ALL"
        :values="filterOptions"
        @checkedValue="setFilterOption"
      />
    </div>
    <watch :opts="opts" spinner noRecordsAlert errorsAlert>
      <template #default="{ items }">
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
              v-for="item in filterItems(items)"
              :key="item.metadata.id!"
              :item="item"
              :searchOption="searchOption"
            />
          </t-group-animation>
        </div>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { getContext } from "@/context";
import { kubernetes, ClusterMachineStatusType, LabelCluster, ClusterMachineStatusLabelNodeName, TalosMemberType, TalosClusterNamespace } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { Resource, ResourceTyped } from "@/api/grpc";
import { ClusterMachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { useRoute } from "vue-router";
import { DefaultNamespace } from "@/api/resources";
import { Node, NodeSpec, NodeStatus } from "kubernetes-types/core/v1";
import { NodesViewFilterOptions } from "@/constants";
import { ref } from "vue";
import { getStatus } from "@/methods";

import Watch from "@/components/common/Watch/Watch.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import NodesItem from "@/views/cluster/Nodes/components/NodesItem.vue";
import TGroupAnimation from "@/components/common/Animation/TGroupAnimation.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";

const filterOption = ref("All");
const searchOption  = ref("");
const setFilterOption = (data: string) => {
  filterOption.value = data;
};

const filterOptions = Object.keys(NodesViewFilterOptions).map(key => NodesViewFilterOptions[key]);

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
  idFunc: (item: ResourceTyped<ClusterMachineStatusSpec>): string => {
    return (item?.metadata?.labels ?? {})[ClusterMachineStatusLabelNodeName] ?? item.metadata.id;
  }
},
{
  runtime: Runtime.Kubernetes,
  resource: {
    type: kubernetes.node,
  },
  context,
  idFunc: (item: Node): string => {
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

type MemberSpec = {
  operatingSystem: string,
  addresses: string[],
}

const filterItems = (items: Resource<ClusterMachineStatusSpec & NodeSpec & MemberSpec, NodeStatus>[]) => {
  return items.filter((elem: any) => {
    const searchFields = [
      (elem?.metadata.labels || {})[ClusterMachineStatusLabelNodeName],
      elem?.metadata.id,
      elem?.spec.operatingSystem,
      (elem?.spec?.addresses ?? {})[0] ?? ""
    ];

    if (getStatus(elem) !== filterOption?.value && filterOption?.value !== NodesViewFilterOptions.ALL) {
      return false;
    }

    if (searchOption?.value === "") {
      return true;
    }

    for (const value of searchFields) {
      if (value.includes(searchOption?.value)) {
        return true;
      }
    }

    return false;
  })
};
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
