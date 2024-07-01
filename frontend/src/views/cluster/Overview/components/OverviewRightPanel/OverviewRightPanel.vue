<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="overview-right-box">
    <div class="overview-right-box-wrapper">
      <h3 class="overview-details-title">Cluster Details</h3>
      <managed-by-templates-warning warning-style="short"/>
      <overview-right-panel-item
        v-if="currentCluster?.metadata?.created"
        name="Created"
        :value="formatISO(currentCluster?.metadata?.created as string, 'yyyy-LL-dd HH:mm:ss')"
      />
      <overview-right-panel-item name="Status" v-if="currentCluster">
        <t-cluster-status :cluster="currentCluster" />
      </overview-right-panel-item>
      <overview-right-panel-item
        name="Machines Healthy"
        v-if="currentCluster?.spec?.machines?.total"
        :value="`${currentCluster?.spec?.machines?.healthy ?? 0}/${currentCluster?.spec?.machines?.total}`"
      />
      <overview-right-panel-item
        v-if="talosUpgradeStatus"
        name="Talos Version"
        @click="openClusterUpdate(Update.Talos)"
      >
        {{ talosVersion }}
        <tooltip v-if="newTalosVersionsAvailable?.length" :description="`Newer Talos versions are avalable: ${newTalosVersionsAvailable.join(', ')}`">
          <t-icon class="text-yellow-Y1 w-4 h-4 cursor-pointer" icon="arrow-up-tray" v-if="newTalosVersionsAvailable"/>
        </tooltip>
      </overview-right-panel-item>
      <overview-right-panel-item
        v-if="kubernetesUpgradeStatus"
        name="Kubernetes Version"
        @click="openClusterUpdate(Update.Kubernetes)"
      >
        {{ kubernetesVersion }}
        <tooltip v-if="newKubernetesVersionsAvailable?.length" :description="`Newer Kubernetes versions are avalable: ${newKubernetesVersionsAvailable.join(', ')}`">
          <t-icon class="text-yellow-Y1 w-4 h-4 cursor-pointer" icon="arrow-up-tray"/>
        </tooltip>
      </overview-right-panel-item>
    </div>
    <template v-if="currentCluster?.spec">
      <div class="divider"/>
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <h3 class="overview-details-title">Control Plane</h3>
        <overview-right-panel-item
          name="Ready">
          <span v-bind:class="currentCluster?.spec?.controlplaneReady ? '' : 'text-red-R1'">
            {{ currentCluster?.spec?.controlplaneReady ? 'Yes' : 'No'  }}
          </span>
        </overview-right-panel-item>
          <overview-right-panel-item
            v-if="currentCluster?.metadata?.created"
            name="Last Backup"
          >
            <div class="flex gap-1" v-if="startingEtcdBackup">
              Starting...
              <t-spinner class="w-4 h-4"/>
            </div>
            <template v-else>
              <tooltip v-if="lastBackupError">
                <template #description>
                  <div class="max-w-lg break-words">{{ lastBackupError }}</div>
                </template>
                <t-icon class="text-yellow-Y1 w-4 h-4" icon="warning"/>
              </tooltip>
              {{ backupTime }}
              <tooltip description="Trigger Etcd Backup" v-if="etcdBackups?.enabled">
                <t-icon class="text-green-G1 w-4 h-4 cursor-pointer" icon="play-circle" @click="runEtcdBackup"/>
              </tooltip>
            </template>
          </overview-right-panel-item>
        <overview-right-panel-condition
          v-for="condition in controlPlaneStatus?.spec.conditions"
          :key="condition.type?.toString()"
          :condition="condition" />
      </div>
      <div class="divider" />
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <h3 class="overview-details-title">Kubernetes</h3>
        <overview-right-panel-item
          name="API Available"
          :value="currentCluster?.spec.kubernetesAPIReady ? 'Yes' : 'No'"
        />
        <overview-right-panel-item
          v-if="kubernetesStatus && kubernetesStatus.spec.nodes"
          name="Nodes"
          :value="kubernetesStatus.spec.nodes.length ?? 0"
        />
      </div>
      <div class="divider"/>
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <div class="overview-details-item">
          <t-button
            @click="() => downloadKubeconfig(currentCluster!.metadata.id!)"
            :disabled="!canDownloadKubeconfig"
            class="overview-item-button w-full"
            type="primary"
            icon="kube-config"
            iconPosition="left"
            >Download <code>kubeconfig</code></t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            @click="() => downloadTalosconfig(currentCluster!.metadata.id!)"
            :disabled="!canDownloadTalosconfig"
            class="overview-item-button w-full"
            type="primary"
            icon="talos-config"
            iconPosition="left"
            >Download <code>talosconfig</code></t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            @click="openDownloadSupportBundle"
            :disabled="!canDownloadSupportBundle"
            class="overview-item-button w-full"
            type="primary"
            icon="lifebuoy"
            iconPosition="left"
          >Download Support Bundle</t-button
          >
        </div>
      </div>
      <div class="divider"/>
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <div class="overview-details-item">
          <t-button
            @click="() => $router.push({ name: 'ClusterScale', params: { cluster: currentCluster?.metadata?.id } })"
            class="overview-item-button w-full"
            type="highlighted"
            icon="nodes"
            iconPosition="left"
            :disabled="!canAddClusterMachines"
            >Cluster Scaling</t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            @click="openClusterUpdate(Update.Kubernetes)"
            class="overview-item-button w-full"
            type="primary"
            icon="kubernetes"
            iconPosition="left"
            :disabled="!canUpdateKubernetes || !kubernetesUpgradeAvailable()"
            >Update Kubernetes</t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            @click="openClusterUpdate(Update.Talos)"
            class="overview-item-button w-full"
            type="primary"
            icon="sidero-monochrome"
            iconPosition="left"
            :disabled="!canUpdateTalos || !talosUpdateAvailable()"
            >Update Talos</t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            @click="() => $router.push({ name: 'ClusterConfigPatches', params: { cluster: currentCluster?.metadata?.id } })"
            class="overview-item-button w-full"
            type="primary"
            icon="settings"
            iconPosition="left"
            >Config Patches</t-button
          >
        </div>
        <div class="overview-details-item">
          <t-button
            class="overview-item-button-red w-full"
            type="secondary"
            icon="delete"
            iconPosition="left"
            @click="() => $router.push({query: { modal: 'clusterDestroy', cluster: currentCluster?.metadata?.id }})"
            :disabled="!canRemoveClusterMachines"
            >Destroy Cluster</t-button
          >
        </div>
      </div>
    </template>
    <div v-else class="flex items-center justify-center p-4">
      <t-spinner class="w-6 h-6"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { computed, Ref, ref, toRefs } from "vue";
import { formatISO } from "@/methods/time";
import { Resource } from "@/api/grpc";
import Watch from "@/api/watch";
import {
  ClusterStatusSpec,
  ControlPlaneStatusSpec,
  KubernetesStatusSpec,
  KubernetesUpgradeStatusSpec,
  TalosUpgradeStatusSpec,
  EtcdBackupStatusSpec,
} from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import {
  ClusterStatusType,
  ControlPlaneStatusType,
  KubernetesStatusType,
  DefaultNamespace,
  EtcdBackupStatusType,
} from "@/api/resources";
import { BackupsStatus, downloadKubeconfig, downloadTalosconfig } from "@/methods";
import { controlPlaneMachineSetId } from "@/methods/machineset";
import * as semver from "semver";

import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TClusterStatus from "@/views/omni/Clusters/ClusterStatus.vue";
import TButton from "@/components/common/Button/TButton.vue";
import OverviewRightPanelItem
  from "@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue";
import OverviewRightPanelCondition
  from "@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelCondition.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import { setupClusterPermissions } from "@/methods/auth";
import { triggerEtcdBackup } from "@/methods/cluster";
import { showError } from "@/notification";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";

const route = useRoute();
const router = useRouter();

const currentCluster: Ref<Resource<ClusterStatusSpec> | undefined> = ref();
const currentClusterWatch = new Watch(currentCluster);

const backupStatus: Ref<Resource<EtcdBackupStatusSpec> | undefined> = ref();
const backupStatusWatch = new Watch(backupStatus);

const controlPlaneStatus: Ref<Resource<ControlPlaneStatusSpec> | undefined> = ref();
const controlPlaneStatusWatch = new Watch(controlPlaneStatus);

const kubernetesStatus: Ref<Resource<KubernetesStatusSpec> | undefined> = ref();
const kubernetesStatusWatch = new Watch(kubernetesStatus);

const props = defineProps<{
  kubernetesUpgradeStatus: Resource<KubernetesUpgradeStatusSpec> | undefined,
  talosUpgradeStatus: Resource<TalosUpgradeStatusSpec> | undefined,
  etcdBackups: BackupsStatus | undefined,
}>();

enum Update {
  Talos,
  Kubernetes
}

const { kubernetesUpgradeStatus, talosUpgradeStatus } = toRefs(props);

currentClusterWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: route.params.cluster as string,
  },
});

controlPlaneStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ControlPlaneStatusType,
    id: controlPlaneMachineSetId(route.params.cluster as string),
  },
});

kubernetesStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesStatusType,
    id: route.params.cluster as string,
  },
});

backupStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: EtcdBackupStatusType,
    id: route.params.cluster as string,
  }
});

const lastBackupError = computed(() => {
  return backupStatus.value?.spec.error;
})

const backupTime = computed(() => {
  const t = backupStatus?.value?.spec.last_backup_time;

  if (!t) {
    return "Never";
  }

  return formatISO(t, 'yyyy-LL-dd HH:mm:ss');
});

const startingEtcdBackup = ref(false);

const runEtcdBackup = async () => {
  startingEtcdBackup.value = true;

  try {
    await triggerEtcdBackup(route.params.cluster as string);
  } catch (e) {
    showError("Failed to Trigger Manual Etcd Backup", e.message);
  }

  startingEtcdBackup.value = false;
}

const kubernetesUpgradeAvailable = () => {
  return (kubernetesUpgradeStatus?.value?.spec?.upgrade_versions?.length ?? 0) > 0
}

const talosUpdateAvailable = () => {
  return (talosUpgradeStatus?.value?.spec?.upgrade_versions?.length ?? 0) > 0
}

const getUpgradeAvailable = (spec: {last_upgrade_version?: string, current_upgrade_version?: string, upgrade_versions?: string[]}) => {
  if (spec.current_upgrade_version || !spec.upgrade_versions || !spec.last_upgrade_version) {
    return [];
  }

  return spec.upgrade_versions.filter((version: string) => {
    return semver.compare(spec.last_upgrade_version!, version) == -1;
  });
}

const kubernetesVersion = computed(() => {
  return getVersion(kubernetesUpgradeStatus.value!.spec);
});

const talosVersion = computed(() => {
  return getVersion(talosUpgradeStatus.value!.spec);
});

const newKubernetesVersionsAvailable = computed(() => {
  return getUpgradeAvailable(kubernetesUpgradeStatus.value!.spec);
});

const newTalosVersionsAvailable = computed(() => {
  return getUpgradeAvailable(talosUpgradeStatus.value!.spec);
});

const getVersion = (spec: {last_upgrade_version?: string, current_upgrade_version?: string}) => {
  if (spec.current_upgrade_version && spec.last_upgrade_version) {
    return `${spec.last_upgrade_version} ⇾ ${spec.current_upgrade_version}`
  }

  return spec.last_upgrade_version;
}

const openClusterUpdate = (type: Update) => {
  if (type === Update.Kubernetes && !canUpdateKubernetes.value) {
    return;
  }

  if (type === Update.Talos && !canUpdateTalos.value) {
    return;
  }

  const modal = type === Update.Talos ? "updateTalos" : "updateKubernetes";

  router.push({ query: { modal: modal, cluster: currentCluster?.value?.metadata?.id } });
}

const openDownloadSupportBundle = () => {
  if (!canDownloadSupportBundle.value) {
    return;
  }

  router.push({ query: { modal: "downloadSupportBundle", cluster: currentCluster?.value?.metadata?.id } });
}

const {
  canUpdateKubernetes,
  canUpdateTalos,
  canDownloadKubeconfig,
  canDownloadTalosconfig,
  canDownloadSupportBundle,
  canAddClusterMachines,
  canRemoveClusterMachines,
} = setupClusterPermissions(computed(() => route.params.cluster as string))
</script>

<style scoped>
.divider {
  @apply w-full bg-naturals-N4 my-3;
  height: 1px;
}

.overview-right-box {
  @apply py-5 bg-naturals-N2 w-full h-auto rounded mb-4;
  max-width: 20%;
  min-width: 270px;
}

.overview-right-box-wrapper {
  @apply flex flex-col lg:px-6 px-2 gap-4;
}

.overview-details-title {
  @apply text-sm text-naturals-N13;
}

.overview-item-button-red {
  @apply text-red-R1;
}

.overview-details-item {
  @apply flex justify-between items-center;
}
</style>
