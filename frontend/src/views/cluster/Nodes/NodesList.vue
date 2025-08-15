<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Node as V1Node } from 'kubernetes-types/core/v1'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
  kubernetes,
  LabelCluster,
  TalosClusterNamespace,
  TalosMemberType,
} from '@/api/resources'
import TGroupAnimation from '@/components/common/Animation/TGroupAnimation.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getContext } from '@/context'
import NodesItem from '@/views/cluster/Nodes/components/NodesItem.vue'

const route = useRoute()
const context = getContext()

const opts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
    },
    selectors: [`${LabelCluster}=${route.params.cluster}`],
    idFunc: (item: Resource<ClusterMachineStatusSpec>): string => {
      return (item?.metadata?.labels ?? {})[ClusterMachineStatusLabelNodeName] ?? item.metadata.id
    },
  },
  {
    runtime: Runtime.Kubernetes,
    resource: {
      type: kubernetes.node,
    },
    context,
    idFunc: (item: V1Node): string => {
      return item.metadata!.name!
    },
  },
  {
    runtime: Runtime.Talos,
    resource: {
      type: TalosMemberType,
      namespace: TalosClusterNamespace,
    },
    context,
    idFunc: (item: any): string => {
      return item.metadata.id
    },
  },
]
</script>

<template>
  <div class="flex w-full flex-col gap-4">
    <PageHeader title="All Nodes" />
    <TList :opts="opts" search pagination>
      <template #default="{ items, searchQuery }">
        <div class="nodes-list">
          <div class="nodes-list-heading">
            <p>Name</p>
            <p>IP</p>
            <p>OS</p>
            <p>Roles</p>
            <p>Status</p>
          </div>
          <TGroupAnimation>
            <NodesItem
              v-for="item in items"
              :key="item.metadata.id!"
              :item="item"
              :search-option="searchQuery"
            />
          </TGroupAnimation>
        </div>
      </template>
    </TList>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.nodes-list-heading {
  @apply flex items-center bg-naturals-n2;
  padding: 10px 16px;
}
.nodes-list-heading > p {
  @apply w-1/5 text-xs text-naturals-n13;
}
</style>
