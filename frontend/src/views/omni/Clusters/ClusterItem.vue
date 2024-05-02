<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="clusters-box">
    <t-slide-down-wrapper :isSliderOpened="!collapsed">
      <template #head>
        <div
          @click="() => { collapsed = !collapsed }"
          class="flex items-center bg-naturals-N1 pl-2 pr-4 py-4 hover:bg-naturals-N3 cursor-pointer"
        >
          <span>
            <t-icon
              class="collapse-button"
              :class="{ 'collapse-button-pushed': collapsed }"
              icon="drop-up"
            />
          </span>
          <div class="clusters-grid flex-1">
            <div class="list-item-link">
              <router-link :to="{ name: 'ClusterOverview', params: { cluster: item?.metadata.id }}">
                <word-highlighter
                  :query="(searchQuery ?? '')"
                  :textToHighlight="item.metadata.id"
                  split-by-space
                  highlightClass="bg-naturals-N14"
                />
              </router-link>
            </div>
            <div class="ml-1.5" id="machine-count">{{ item?.spec?.machines?.healthy ?? 0 }}/{{ item?.spec?.machines?.total ?? 0 }}</div>
            <div/>
            <item-labels v-if="item" :resource="item" :add-label-func="addClusterLabels"
              :remove-label-func="removeClusterLabels"
              @filter-label="(e) => $emit('filterLabels', e)"/>
          </div>
          <tooltip description="Open Cluster Dashboard" class="h-11">
            <icon-button icon="dashboard" @click.stop="() => $router.push({ path: '/cluster/' + item?.metadata.id + '/overview' })"/>
          </tooltip>
          <t-actions-box style="height: 24px" @click.stop>
            <t-actions-box-item
              icon="nodes"
              @click="() => $router.push({ name: 'ClusterScale', params: { cluster: item?.metadata?.id } })"
              v-if="canAddClusterMachines"
            >Cluster Scaling
            </t-actions-box-item>
            <t-actions-box-item
              icon="settings"
              @click="() => $router.push({ name: 'ClusterConfigPatches', params: { cluster: item?.metadata?.id } })"
            >Config Patches
            </t-actions-box-item>
            <t-actions-box-item
              icon="dashboard"
              @click="() => $router.push({ name: 'ClusterOverview', params: { cluster: item?.metadata?.id } })"
            >Open Dashboard
            </t-actions-box-item>
            <t-actions-box-item
              icon="kube-config"
              @click="() => downloadKubeconfig(item?.metadata?.id as string)"
              v-if="canDownloadKubeconfig"
            >Download <code>kubeconfig</code></t-actions-box-item>
            <t-actions-box-item
              icon="talos-config"
              @click="() => downloadTalosconfig(item?.metadata?.id)"
              v-if="canDownloadTalosconfig"
            >Download <code>talosconfig</code></t-actions-box-item>
            <t-actions-box-item icon="delete" danger @click="openClusterDestroy" v-if="canRemoveClusterMachines">Destroy Cluster</t-actions-box-item>
          </t-actions-box>
        </div>
      </template>
      <template #body v-if="!collapsed">
        <cluster-machines :clusterID="item.metadata.id!"/>
      </template>
    </t-slide-down-wrapper>
  </div>
</template>

<script setup lang="ts">
import { computed, toRefs, ref, Ref } from "vue";
import {
  DefaultNamespace,
  LabelCluster,
  LabelMachineSet,
  MachineSetNodeType, MachineSetType,
} from "@/api/resources";
import { downloadKubeconfig, downloadTalosconfig } from "@/methods";
import { useRouter } from "vue-router";
import { addClusterLabels, removeClusterLabels } from "@/methods/cluster";
import { controlPlaneMachineSetId } from "@/methods/machineset";
import { Resource } from "@/api/grpc";
import { ClusterStatusSpec, MachineSetSpec } from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import storageRef from "@/methods/storage";
import WatchResource from "@/api/watch";

import TIcon from "@/components/common/Icon/TIcon.vue";
import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";
import TSlideDownWrapper from "@/components/common/SlideDownWrapper/TSlideDownWrapper.vue";
import ClusterMachines from "@/views/cluster/ClusterMachines/ClusterMachines.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import WordHighlighter from "vue-word-highlighter";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import { setupClusterPermissions } from "@/methods/auth";

const props = defineProps<{
  item: Resource<ClusterStatusSpec>,
  defaultOpen?: boolean,
  searchQuery?: string
}>();

defineEmits(["filterLabels"]);

const { item } = toRefs(props);

const router = useRouter();
const collapsed = storageRef(sessionStorage, `cluster-collapsed-${item?.value?.metadata?.id}`, !props.defaultOpen);

const openClusterDestroy = () => {
  router.push({
    query: { modal: "clusterDestroy", cluster: item.value.metadata.id },
  });
};

const machineSets: Ref<Resource<MachineSetSpec>[]> = ref([]);
const machineSetsWatch = new WatchResource(machineSets);
machineSetsWatch.setup(computed(() => {
  return {
    resource: {
      type: MachineSetType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
    selectors: [`${LabelCluster}=${item.value.metadata.id!}`]
  };
}))

const { canDownloadKubeconfig, canDownloadTalosconfig, canAddClusterMachines, canRemoveClusterMachines } = setupClusterPermissions(computed(() => item.value.metadata.id!))

const controlPlaneNodes = ref<Resource[]>([]);

const machineNodesWatch = new WatchResource(controlPlaneNodes);
machineNodesWatch.setup(computed(() => {
  return {
    resource: {
      type: MachineSetNodeType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
    selectors: [
      `${LabelMachineSet}=${controlPlaneMachineSetId(item.value.metadata.id!)}`
    ]
  };
}))
</script>

<style>
.clusters-grid {
  @apply items-center grid grid-cols-4 pr-2;
}

.clusters-grid > * {
  @apply text-xs truncate;
}

.clusters-box {
  @apply border border-naturals-N5 rounded overflow-hidden;
}

.collapse-button {
  @apply fill-current text-naturals-N11 hover:bg-naturals-N7 transition-all rounded duration-300 cursor-pointer mr-1;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}

.collapse-button-pushed {
  transform: rotate(0);
}
</style>
