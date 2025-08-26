<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onMounted, ref, toRefs, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterSpec,
  ClusterStatusSpec,
  KubernetesUpgradeStatusSpec,
  KubernetesUsageSpec,
  TalosUpgradeStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  KubernetesUpgradeStatusSpecPhase,
  TalosUpgradeStatusSpecPhase,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterStatusType,
  DefaultNamespace,
  KubernetesUpgradeStatusType,
  KubernetesUsageType,
  TalosUpgradeStatusType,
  VirtualNamespace,
} from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import RadialBar from '@/components/common/Charts/RadialBar.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { getContext } from '@/context'
import { formatBytes, setupBackupStatus } from '@/methods'
import { setupClusterPermissions } from '@/methods/auth'
import {
  addClusterLabels,
  removeClusterLabels,
  revertKubernetesUpgrade,
  revertTalosUpgrade,
  setClusterEtcdBackupsConfig,
  setClusterWorkloadProxy,
  setUseEmbeddedDiscoveryService,
} from '@/methods/cluster'
import {
  embeddedDiscoveryServiceFeatureAvailable,
  setupWorkloadProxyingEnabledFeatureWatch,
} from '@/methods/features'
import ClusterMachines from '@/views/cluster/ClusterMachines/ClusterMachines.vue'
import OverviewRightPanel from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanel.vue'
import ClusterEtcdBackupCheckbox from '@/views/omni/Clusters/ClusterEtcdBackupCheckbox.vue'
import ClusterWorkloadProxyingCheckbox from '@/views/omni/Clusters/ClusterWorkloadProxyingCheckbox.vue'
import EmbeddedDiscoveryServiceCheckbox from '@/views/omni/Clusters/EmbeddedDiscoveryServiceCheckbox.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'

// Do not show stats if the cluster has more than this number of machines.
// Because it overloads the UI and the backend for no good reason.
const clusterSizeStatsThreshold = 50

type Props = {
  currentCluster: Resource<ClusterSpec>
}

const props = defineProps<Props>()
const { currentCluster } = toRefs(props)

const enableWorkloadProxy = ref(currentCluster.value.spec.features?.enable_workload_proxy || false)
const useEmbeddedDiscoveryService = ref(
  currentCluster.value.spec.features?.use_embedded_discovery_service || false,
)

watch(currentCluster, (cluster) => {
  enableWorkloadProxy.value = cluster.spec.features?.enable_workload_proxy || false
  useEmbeddedDiscoveryService.value = cluster.spec.features?.use_embedded_discovery_service || false
})

const { status: backupStatus } = setupBackupStatus()

const context = getContext()

const kubernetesUpgradeStatus: Ref<Resource<KubernetesUpgradeStatusSpec> | undefined> = ref()
const kubernetesUpgradeStatusWatch = new Watch(kubernetesUpgradeStatus)

kubernetesUpgradeStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeStatusType,
    id: context.cluster,
  },
})

const clusterStatus: Ref<Resource<ClusterStatusSpec> | undefined> = ref()

const usage: Ref<Resource<KubernetesUsageSpec> | undefined> = ref()

const usageWatch = new Watch(usage)

const clusterStatusWatch = new Watch(clusterStatus)

clusterStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: context.cluster,
  },
})

const showStats = computed(() => {
  return (clusterStatus.value?.spec.machines?.total ?? 0) < clusterSizeStatsThreshold
})

usageWatch.setup(
  computed(() => {
    if (!clusterStatus.value) {
      return
    }

    if (!showStats.value) {
      return
    }

    return {
      runtime: Runtime.Omni,
      resource: {
        namespace: VirtualNamespace,
        type: KubernetesUsageType,
        id: context.cluster,
      },
    }
  }),
)

const getNumber = (value?: number): number => {
  return value ?? 0
}

const talosUpgradeStatus: Ref<Resource<TalosUpgradeStatusSpec> | undefined> = ref()
const talosUpgradeStatusWatch = new Watch(talosUpgradeStatus)

talosUpgradeStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: context.cluster,
  },
})

const { canManageClusterFeatures } = setupClusterPermissions(
  computed(() => currentCluster.value.metadata.id as string),
)

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch()

const isEmbeddedDiscoveryServiceAvailable = ref(false)

const toggleUseEmbeddedDiscoveryService = async () => {
  const newValue = isEmbeddedDiscoveryServiceAvailable.value
    ? !useEmbeddedDiscoveryService.value
    : false

  await setUseEmbeddedDiscoveryService(context.cluster ?? '', newValue)
}

onMounted(async () => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceFeatureAvailable()
})
</script>

<template>
  <div>
    <div class="overview">
      <div class="overview-container">
        <div class="overview-charts-box relative">
          <div
            class="flex w-full flex-wrap items-stretch justify-around gap-2 transition-opacity duration-500"
            :class="{ 'opacity-25': !showStats }"
          >
            <RadialBar
              name="CPU"
              :total="getNumber(usage?.spec?.cpu?.capacity)"
              :labels="['Requests', 'Limits']"
              :series="[getNumber(usage?.spec?.cpu?.requests), getNumber(usage?.spec?.cpu?.limits)]"
            />
            <RadialBar
              name="Pods"
              :total="getNumber(usage?.spec?.pods?.capacity)"
              :labels="['Requests']"
              :series="[getNumber(usage?.spec?.pods?.count)]"
              :formatter="(value: number) => value.toFixed(0)"
            />
            <RadialBar
              name="Memory"
              :total="getNumber(usage?.spec?.mem?.capacity)"
              :labels="['Requests', 'Limits']"
              :series="[getNumber(usage?.spec?.mem?.requests), getNumber(usage?.spec?.mem?.limits)]"
              :formatter="formatBytes"
            />
            <RadialBar
              name="Ephemeral Storage"
              :total="getNumber(usage?.spec?.storage?.capacity)"
              :labels="['Requests', 'Limits']"
              :series="[
                getNumber(usage?.spec?.storage?.requests),
                getNumber(usage?.spec?.storage?.limits),
              ]"
              :formatter="formatBytes"
            />
          </div>
          <div
            v-if="!showStats"
            class="absolute top-0 right-0 bottom-0 left-0 flex items-center justify-center text-sm"
          >
            <div class="flex flex-col gap-2">
              <div class="text-naturals-n13">
                Kubernetes stats are disabled due to the size of the cluster
              </div>
            </div>
          </div>
        </div>
        <div
          v-if="kubernetesUpgradeStatus && kubernetesUpgradeStatus.spec.step"
          class="overview-upgrade-progress"
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
              <span class="overview-box-title"> Reverting back to </span>
              <span class="overview-upgrade-version">
                {{ kubernetesUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
          </div>
          <div class="flex items-center gap-2 border-t-8 border-naturals-n4 p-4 text-xs">
            <TIcon icon="loading" class="h-6 w-6 animate-spin text-yellow-y1" />
            <div class="flex-1">
              {{ kubernetesUpgradeStatus.spec.step }}
              <template v-if="kubernetesUpgradeStatus.spec.status">
                - {{ kubernetesUpgradeStatus.spec.status }}
              </template>
            </div>
            <TButton
              v-if="
                kubernetesUpgradeStatus.spec.phase === KubernetesUpgradeStatusSpecPhase.Upgrading
              "
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertKubernetesUpgrade(context.cluster ?? '')"
              >Cancel</TButton
            >
          </div>
        </div>
        <div
          v-if="talosUpgradeStatus && talosUpgradeStatus.spec.status"
          class="overview-upgrade-progress"
        >
          <div class="overview-box-header flex items-center gap-1">
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
            <template
              v-else-if="talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Reverting"
            >
              <span class="overview-box-title"> Reverting back to </span>
              <span class="overview-upgrade-version">
                {{ talosUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
            <template
              v-else-if="
                talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.InstallingExtensions
              "
            >
              <span class="overview-box-title"> Installing Extensions </span>
            </template>
          </div>
          <div class="flex items-center gap-2 border-t-8 border-naturals-n4 p-4 text-xs">
            <TIcon icon="loading" class="h-6 w-6 animate-spin text-yellow-y1" />
            <div class="flex-1">
              {{ talosUpgradeStatus.spec.status }}
              <template v-if="talosUpgradeStatus.spec.status">
                - {{ talosUpgradeStatus.spec.step }}
              </template>
            </div>
            <TButton
              v-if="
                talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Upgrading &&
                talosUpgradeStatus.spec.current_upgrade_version
              "
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertTalosUpgrade(context.cluster ?? '')"
              >Cancel</TButton
            >
          </div>
        </div>
        <div class="flex gap-5">
          <div v-if="workloadProxyingEnabled" class="overview-card mb-5 flex-1 px-6">
            <div class="mb-3">
              <span class="overview-box-title">Features</span>
            </div>
            <div class="flex flex-col gap-2">
              <ClusterWorkloadProxyingCheckbox
                :checked="enableWorkloadProxy"
                :disabled="!canManageClusterFeatures"
                @click="setClusterWorkloadProxy(context.cluster ?? '', !enableWorkloadProxy)"
              />
              <EmbeddedDiscoveryServiceCheckbox
                :checked="useEmbeddedDiscoveryService"
                :disabled="!canManageClusterFeatures || !isEmbeddedDiscoveryServiceAvailable"
                @click="toggleUseEmbeddedDiscoveryService"
              />
              <ClusterEtcdBackupCheckbox
                :backup-status="backupStatus"
                :cluster="currentCluster.spec"
                @update:cluster="(spec) => setClusterEtcdBackupsConfig(context.cluster ?? '', spec)"
              />
            </div>
          </div>
          <div class="overview-card mb-5 flex-1 px-6">
            <div class="mb-3">
              <span class="overview-box-title">Labels</span>
            </div>
            <ItemLabels
              :resource="currentCluster"
              :add-label-func="addClusterLabels"
              :remove-label-func="removeClusterLabels"
            />
          </div>
        </div>
        <div class="overview-card overview-machines-list">
          <div class="overview-box-header">
            <span class="overview-box-title">Machines</span>
          </div>
          <ClusterMachines :cluster-i-d="context.cluster!" />
        </div>
      </div>
      <OverviewRightPanel
        :kubernetes-upgrade-status="kubernetesUpgradeStatus"
        :talos-upgrade-status="talosUpgradeStatus"
        :etcd-backups="backupStatus"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../index.css";

.overview-card {
  @apply rounded bg-naturals-n2 py-5;
}
.divider {
  @apply w-full bg-naturals-n4;
  height: 1px;
}
.overview {
  @apply flex w-full items-start justify-start;
}
.overview-container {
  @apply mr-6 w-full;
  max-width: 80%;
}
@media screen and (max-width: 1050px) {
  .overview-container {
    @apply mr-2 w-full;
  }
}
.overview-title-box {
  @apply flex items-center;
  margin-bottom: 35px;
}
.overview-title {
  @apply mr-2 text-xl text-naturals-n14;
}
.overview-icon {
  @apply h-5 w-5 cursor-pointer fill-current text-naturals-n14;
}
.overview-machines-list {
  @apply flex-col;
  padding-bottom: 0;
}
.overview-kubernetes-upgrade {
  @apply mb-5 rounded bg-naturals-n2 py-5 pb-0;
}
.overview-upgrade-progress {
  @apply mb-5 rounded bg-naturals-n2 py-5 pb-0;
}
.overview-box-header {
  @apply flex px-6 pb-4;
}
.overview-box-title {
  @apply text-sm text-naturals-n13;
}
.overview-usage-subtitle {
  @apply text-xs text-naturals-n10;
}
.overview-status-box {
  @apply w-full flex-col rounded bg-naturals-n2 py-5 pb-0;
}
.overview-charts-box {
  @apply mb-6 bg-naturals-n2 p-5;
  min-height: 100px;
}
.overview-upgrade-version {
  @apply rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13;
}
</style>
