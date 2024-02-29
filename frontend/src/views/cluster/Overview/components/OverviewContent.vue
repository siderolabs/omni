<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <div class="overview">
      <div class="overview-container">
        <div class="overview-charts-box relative">
          <div class="flex flex-wrap justify-around w-full items-stretch gap-2">
            <radial-bar
              name="CPU"
              :total="getNumber(usage?.spec?.cpu?.capacity)"
              :labels="[
                'Requests',
                'Limits',
              ]"
              :series="[
                getNumber(usage?.spec?.cpu?.requests),
                getNumber(usage?.spec?.cpu?.limits)
              ]"
            />
            <radial-bar
              name="Pods"
              :total="getNumber(usage?.spec?.pods?.capacity)"
              :labels="[
                'Requests',
              ]"
              :series="[
                getNumber(usage?.spec?.pods?.count)
              ]"
              :formatter="(value: number) => value.toFixed(0)"
            />
            <radial-bar
              name="Memory"
              :total="getNumber(usage?.spec?.mem?.capacity)"
              :labels="[
                'Requests',
                'Limits',
              ]"
              :series="[
                getNumber(usage?.spec?.mem?.requests),
                getNumber(usage?.spec?.mem?.limits)
              ]"
              :formatter="formatBytes"
            />
            <radial-bar
              name="Ephemeral Storage"
              :total="getNumber(usage?.spec?.storage?.capacity)"
              :labels="[
                'Requests',
                'Limits',
              ]"
              :series="[
                getNumber(usage?.spec?.storage?.requests),
                getNumber(usage?.spec?.storage?.limits)
              ]"
              :formatter="formatBytes"
            />
          </div>
        </div>
        <div
          class="overview-upgrade-progress"
          v-if="kubernetesUpgradeStatus && kubernetesUpgradeStatus.spec.step"
        >
          <div class="overview-box-header flex items-center gap-1">
            <span class="overview-box-title flex-1">Kubernetes Update</span>
            <template v-if="kubernetesUpgradeStatus.spec.current_upgrade_version">
              <span class="overview-upgrade-version">
                {{ kubernetesUpgradeStatus.spec.last_upgrade_version }}
              </span>
              <span>⇾</span>
              <span class="overview-upgrade-version">
                {{ kubernetesUpgradeStatus.spec.current_upgrade_version }}
              </span>
            </template>
            <template v-else>
              <span class="overview-box-title">
                Reverting back to
              </span>
              <span class="overview-upgrade-version">
                {{ kubernetesUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
          </div>
          <div class="border-t-8 border-naturals-N4 p-4 text-xs flex items-center gap-2">
            <t-icon icon="loading" class="animate-spin w-6 h-6 text-yellow-Y1"/>
            <div class="flex-1">
              {{ kubernetesUpgradeStatus.spec.step }}
              <template v-if="kubernetesUpgradeStatus.spec.status">
                - {{ kubernetesUpgradeStatus.spec.status }}
              </template>
            </div>
            <t-button
              v-if="kubernetesUpgradeStatus.spec.phase === KubernetesUpgradeStatusSpecPhase.Upgrading"
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertKubernetesUpgrade(context.cluster)"
              >Cancel</t-button
            >
          </div>
        </div>
        <div
          class="overview-upgrade-progress"
          v-if="talosUpgradeStatus && talosUpgradeStatus.spec.status"
        >
          <div class="overview-box-header flex gap-1 items-center">
            <span class="overview-box-title flex-1">Talos Update</span>
            <template v-if="talosUpgradeStatus.spec.current_upgrade_version">
              <span class="overview-upgrade-version">
                {{ talosUpgradeStatus.spec.last_upgrade_version }}
              </span>
              <span>⇾</span>
              <span class="overview-upgrade-version">
                {{ talosUpgradeStatus.spec.current_upgrade_version }}
              </span>
            </template>
            <template v-else-if="talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Reverting">
              <span class="overview-box-title">
                Reverting back to
              </span>
              <span class="overview-upgrade-version">
                {{ talosUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
            <template v-else>
              <span class="overview-box-title">
                Installing Extensions
              </span>
            </template>
          </div>
          <div class="border-t-8 border-naturals-N4 p-4 text-xs flex items-center gap-2">
            <t-icon icon="loading" class="animate-spin w-6 h-6 text-yellow-Y1"/>
            <div class="flex-1">
              {{ talosUpgradeStatus.spec.status }}
              <template v-if="talosUpgradeStatus.spec.status">
                - {{ talosUpgradeStatus.spec.step }}
              </template>
            </div>
            <t-button
              v-if="talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Upgrading"
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertTalosUpgrade(context.cluster)"
              >Cancel</t-button
            >
          </div>
        </div>
        <div class="flex gap-5">
          <div class="overview-card flex-1 mb-5 px-6" v-if="workloadProxyingEnabled">
            <div class="mb-3">
              <span class="overview-box-title">Features</span>
            </div>
            <div class="flex flex-col gap-2">
              <cluster-workload-proxying-checkbox :checked="enableWorkloadProxy" @click="setClusterWorkloadProxy(context.cluster, !enableWorkloadProxy)" :disabled="!canManageClusterFeatures"/>
              <cluster-etcd-backup-checkbox :backup-status="backupStatus" @update:cluster="(spec) => setClusterEtcdBackupsConfig(context.cluster, spec)" :cluster="currentCluster.spec"/>
            </div>
          </div>
          <div class="overview-card flex-1 mb-5 px-6">
            <div class="mb-3">
              <span class="overview-box-title">Labels</span>
            </div>
            <item-labels :resource="currentCluster" :add-label-func="addClusterLabels"
              :remove-label-func="removeClusterLabels"/>
          </div>
        </div>
        <div class="overview-machines-list">
          <div class="overview-box-header">
            <span class="overview-box-title">Machines</span>
          </div>
          <cluster-machines :clusterID="context.cluster"/>
        </div>
      </div>
      <overview-right-panel
        :kubernetes-upgrade-status="kubernetesUpgradeStatus"
        :talos-upgrade-status="talosUpgradeStatus"
        :etcd-backups="backupStatus"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, Ref, toRefs, watch } from "vue";
import { getContext } from "@/context";
import { formatBytes, setupBackupStatus } from "@/methods";
import { KubernetesUsageType, VirtualNamespace, TalosUpgradeStatusType } from "@/api/resources";
import { Resource } from "@/api/grpc";
import Watch from "@/api/watch";
import { Runtime } from "@/api/common/omni.pb";
import {
  addClusterLabels,
  removeClusterLabels,
  revertKubernetesUpgrade,
  revertTalosUpgrade,
  setClusterWorkloadProxy,
  setClusterEtcdBackupsConfig,
} from "@/methods/cluster";
import {
  ClusterSpec,
  KubernetesUpgradeStatusSpec,
  KubernetesUpgradeStatusSpecPhase,
  KubernetesUsageSpec,
  TalosUpgradeStatusSpec,
  TalosUpgradeStatusSpecPhase
} from "@/api/omni/specs/omni.pb";
import { KubernetesUpgradeStatusType, DefaultNamespace } from "@/api/resources";

import ClusterMachines from "@/views/cluster/ClusterMachines/ClusterMachines.vue";
import OverviewRightPanel from "@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanel.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import RadialBar from "@/components/common/Charts/RadialBar.vue";
import { setupClusterPermissions } from "@/methods/auth";

import { setupWorkloadProxyingEnabledFeatureWatch } from "@/methods/features";
import ClusterWorkloadProxyingCheckbox from "@/views/omni/Clusters/ClusterWorkloadProxyingCheckbox.vue";
import ClusterEtcdBackupCheckbox from "@/views/omni/Clusters/ClusterEtcdBackupCheckbox.vue";

type Props = {
  currentCluster: Resource<ClusterSpec>,
};

const props = defineProps<Props>()
const { currentCluster } = toRefs(props);

const enableWorkloadProxy = ref(currentCluster.value.spec.features?.enable_workload_proxy || false);

watch(currentCluster, (cluster) => {
  enableWorkloadProxy.value = cluster.spec.features?.enable_workload_proxy || false;
});

const { status: backupStatus } = setupBackupStatus();

const context = getContext();

const kubernetesUpgradeStatus: Ref<
  Resource<KubernetesUpgradeStatusSpec> | undefined
> = ref();
const kubernetesUpgradeStatusWatch = new Watch(kubernetesUpgradeStatus);

kubernetesUpgradeStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeStatusType,
    id: context.cluster,
  },
});

const usage: Ref<Resource<KubernetesUsageSpec> | undefined> = ref();

const usageWatch = new Watch(usage);

usageWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: KubernetesUsageType,
    id: context.cluster,
  },
});

const getNumber = (value?: number): number => {
  return value ?? 0;
}

const talosUpgradeStatus: Ref<
  Resource<TalosUpgradeStatusSpec> | undefined
> = ref();
const talosUpgradeStatusWatch = new Watch(talosUpgradeStatus);

talosUpgradeStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: context.cluster,
  },
});

const { canManageClusterFeatures } = setupClusterPermissions(computed(() => currentCluster.value.metadata.id as string));

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch();
</script>

<style scoped>
.overview-card {
    @apply py-5 bg-naturals-N2 rounded;
}
.divider {
  @apply w-full bg-naturals-N4;
  height: 1px;
}
.overview {
  @apply w-full flex justify-start items-start;
}
.overview-container {
  @apply w-full mr-6;
  max-width: 80%;
}
@media screen and (max-width: 1050px) {
  .overview-container {
    @apply w-full mr-2;
  }
}
.overview-title-box {
  @apply flex items-center;
  margin-bottom: 35px;
}
.overview-title {
  @apply text-xl text-naturals-N14 mr-2;
}
.overview-icon {
  @apply fill-current text-naturals-N14 w-5 h-5 cursor-pointer;
}
.overview-machines-list {
  @apply flex-col overview-card;
  padding-bottom: 0;
}
.overview-kubernetes-upgrade {
  @apply mb-5 py-5 bg-naturals-N2 pb-0 rounded;
}
.overview-upgrade-progress {
  @apply mb-5 py-5 bg-naturals-N2 pb-0 rounded;
}
.overview-box-header {
  @apply flex pb-4 px-6;
}
.overview-box-title {
  @apply text-sm text-naturals-N13;
}
.overview-usage-subtitle {
  @apply text-xs text-naturals-N10;
}
.overview-status-box {
  @apply w-full bg-naturals-N2 py-5 rounded flex-col pb-0;
}
.overview-charts-box {
  @apply p-5 bg-naturals-N2 mb-6;
  min-height: 100px;
}
.overview-upgrade-version {
  @apply rounded bg-naturals-N4 px-2 text-sm text-naturals-N13 font-bold;
}
</style>
