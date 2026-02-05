<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, onMounted, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import {
  type ClusterSecretsRotationStatusSpec,
  type ClusterSpec,
  type ClusterStatusSpec,
  type KubernetesUpgradeStatusSpec,
  KubernetesUpgradeStatusSpecPhase,
  type KubernetesUsageSpec,
  SecretRotationSpecComponent,
  type TalosUpgradeStatusSpec,
  TalosUpgradeStatusSpecPhase,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterLocked,
  ClusterSecretsRotationStatusType,
  ClusterStatusType,
  DefaultNamespace,
  KubernetesUpgradeStatusType,
  KubernetesUsageType,
  TalosUpgradeStatusType,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import RadialBar from '@/components/common/Charts/RadialBar.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TAlert from '@/components/TAlert.vue'
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
import { embeddedDiscoveryServiceFeatureAvailable, useFeatures } from '@/methods/features'
import { useResourceWatch } from '@/methods/useResourceWatch'
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

const { currentCluster } = defineProps<Props>()

const enableWorkloadProxy = ref(false)
const useEmbeddedDiscoveryService = ref(false)

watchEffect(() => {
  enableWorkloadProxy.value = currentCluster.spec.features?.enable_workload_proxy || false
  useEmbeddedDiscoveryService.value =
    currentCluster.spec.features?.use_embedded_discovery_service || false
})

const { status: backupStatus } = setupBackupStatus()

const clusterId = computed(() => currentCluster.metadata.id ?? '')

const { data: kubernetesUpgradeStatus } = useResourceWatch<KubernetesUpgradeStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeStatusType,
    id: clusterId.value,
  },
}))

const { data: clusterStatus } = useResourceWatch<ClusterStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: clusterId.value,
  },
}))

const showStats = computed(() => {
  return (clusterStatus.value?.spec.machines?.total ?? 0) < clusterSizeStatsThreshold
})

const { data: usage } = useResourceWatch<KubernetesUsageSpec>(() => ({
  skip: !clusterStatus.value || !showStats.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: KubernetesUsageType,
    id: clusterId.value,
  },
}))

const { data: talosUpgradeStatus } = useResourceWatch<TalosUpgradeStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: clusterId.value,
  },
}))

const { canManageClusterFeatures } = setupClusterPermissions(clusterId)

const { data: features } = useFeatures()

const isEmbeddedDiscoveryServiceAvailable = ref(false)

const toggleUseEmbeddedDiscoveryService = async (value: boolean) => {
  await setUseEmbeddedDiscoveryService(clusterId.value, value)
}

const { data: secretRotationStatus } = useResourceWatch<ClusterSecretsRotationStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterSecretsRotationStatusType,
    id: clusterId.value,
  },
}))

const getComponentInRotation = computed(() => {
  switch (secretRotationStatus.value?.spec.component) {
    case SecretRotationSpecComponent.TALOS_CA:
      return 'Talos CA'
    default:
      return ''
  }
})

const clusterLocked = computed(() => {
  return currentCluster.metadata.annotations?.[ClusterLocked] !== undefined
})

const machineLockedForTalosUpgrade = computed(() => {
  return talosUpgradeStatus.value?.spec.status === 'upgrade paused'
})

const machineLockedForKubernetesUpgrade = computed(() => {
  return kubernetesUpgradeStatus.value?.spec.status === 'waiting for machine to be unlocked'
})

const machineLockedForSecretRotation = computed(() => {
  return secretRotationStatus.value?.spec.status === 'rotation paused'
})

onMounted(async () => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceFeatureAvailable()
})
</script>

<template>
  <div>
    <div class="flex w-full items-start justify-start">
      <div class="mr-6 w-full max-w-[80%] max-lg:mr-2 max-lg:w-full">
        <TAlert v-if="clusterLocked" title="Cluster is Locked" type="warn" class="mb-4">
          All operations on this cluster are currently disabled. Config patches can be created,
          updated or deleted but these changes will not be applied while the cluster is locked.
        </TAlert>
        <div class="relative mb-6 min-h-25 bg-naturals-n2 p-5">
          <div
            class="flex w-full flex-wrap gap-2 transition-opacity duration-500 *:flex-1"
            :class="{ 'opacity-25': !showStats }"
          >
            <RadialBar
              title="CPU"
              vertical
              :total="usage?.spec?.cpu?.capacity ?? 0"
              :items="[
                { label: 'Requests', value: usage?.spec?.cpu?.requests ?? 0 },
                { label: 'Limits', value: usage?.spec?.cpu?.limits ?? 0 },
              ]"
              :legend-formatter="(input) => input.toFixed(2)"
            />
            <RadialBar
              title="Pods"
              vertical
              :total="usage?.spec?.pods?.capacity ?? 0"
              :items="[{ label: 'Requests', value: usage?.spec?.pods?.count ?? 0 }]"
            />
            <RadialBar
              title="Memory"
              vertical
              :total="usage?.spec?.mem?.capacity ?? 0"
              :items="[
                { label: 'Requests', value: usage?.spec?.mem?.requests ?? 0 },
                { label: 'Limits', value: usage?.spec?.mem?.limits ?? 0 },
              ]"
              :legend-formatter="formatBytes"
            />
            <RadialBar
              title="Ephemeral Storage"
              vertical
              :total="usage?.spec?.storage?.capacity ?? 0"
              :items="[
                { label: 'Requests', value: usage?.spec?.storage?.requests ?? 0 },
                { label: 'Limits', value: usage?.spec?.storage?.limits ?? 0 },
              ]"
              :legend-formatter="formatBytes"
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
          class="mb-5 rounded bg-naturals-n2 pt-5"
        >
          <div class="flex items-center gap-1 px-6 pb-4">
            <span class="flex-1 text-sm text-naturals-n13">Kubernetes Update</span>
            <template v-if="kubernetesUpgradeStatus.spec.current_upgrade_version">
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ kubernetesUpgradeStatus.spec.last_upgrade_version }}
              </span>
              <span>⇾</span>
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ kubernetesUpgradeStatus.spec.current_upgrade_version }}
              </span>
            </template>
            <template v-else>
              <span class="text-sm text-naturals-n13">Reverting back to</span>
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ kubernetesUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
          </div>
          <div class="flex min-h-20 items-center gap-2 border-t-8 border-naturals-n4 p-4 text-xs">
            <TIcon
              v-if="clusterLocked || machineLockedForKubernetesUpgrade"
              icon="pause-circle"
              class="h-6 w-6"
            />
            <TIcon v-else icon="loading" class="h-6 w-6 animate-spin text-yellow-y1" />
            <div class="flex-1">
              {{ kubernetesUpgradeStatus.spec.step }}
              <template v-if="kubernetesUpgradeStatus.spec.status && !clusterLocked">
                - {{ kubernetesUpgradeStatus.spec.status }}
              </template>
              <template v-if="clusterLocked">- waiting for cluster to be unlocked</template>
            </div>
            <TButton
              v-if="
                kubernetesUpgradeStatus.spec.phase === KubernetesUpgradeStatusSpecPhase.Upgrading &&
                !clusterLocked
              "
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertKubernetesUpgrade(clusterId)"
            >
              Cancel
            </TButton>
          </div>
        </div>
        <div
          v-if="talosUpgradeStatus && talosUpgradeStatus.spec.status"
          class="mb-5 rounded bg-naturals-n2 pt-5"
        >
          <div class="flex items-center gap-1 px-6 pb-4">
            <span class="flex-1 text-sm text-naturals-n13">Talos Update</span>
            <template v-if="talosUpgradeStatus.spec.current_upgrade_version">
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ talosUpgradeStatus.spec.last_upgrade_version }}
              </span>
              <span>⇾</span>
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ talosUpgradeStatus.spec.current_upgrade_version }}
              </span>
            </template>
            <template
              v-else-if="talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Reverting"
            >
              <span class="text-sm text-naturals-n13">Reverting back to</span>
              <span class="rounded bg-naturals-n4 px-2 text-sm font-bold text-naturals-n13">
                {{ talosUpgradeStatus.spec.last_upgrade_version }}
              </span>
            </template>
            <template
              v-else-if="
                talosUpgradeStatus.spec.phase ===
                TalosUpgradeStatusSpecPhase.UpdatingMachineSchematics
              "
            >
              <span class="text-sm text-naturals-n13">Updating Machine Schematics</span>
            </template>
          </div>
          <div class="flex min-h-20 items-center gap-2 border-t-8 border-naturals-n4 p-4 text-xs">
            <TIcon
              v-if="clusterLocked || machineLockedForTalosUpgrade"
              icon="pause-circle"
              class="h-6 w-6"
            />
            <TIcon v-else icon="loading" class="h-6 w-6 animate-spin text-yellow-y1" />
            <div class="flex-1">
              {{ talosUpgradeStatus.spec.status }}
              <template v-if="talosUpgradeStatus.spec.status && !clusterLocked">
                - {{ talosUpgradeStatus.spec.step }}
              </template>
              <template v-if="clusterLocked">- waiting for cluster to be unlocked</template>
            </div>
            <TButton
              v-if="
                talosUpgradeStatus.spec.phase === TalosUpgradeStatusSpecPhase.Upgrading &&
                talosUpgradeStatus.spec.current_upgrade_version &&
                !clusterLocked
              "
              type="secondary"
              class="place-self-end"
              icon="close"
              @click="revertTalosUpgrade(clusterId)"
            >
              Cancel
            </TButton>
          </div>
        </div>
        <div
          v-if="secretRotationStatus && secretRotationStatus.spec.status"
          class="mb-5 rounded bg-naturals-n2 pt-5"
        >
          <div class="flex items-center gap-1 px-6 pb-4">
            <span class="flex-1 text-sm text-naturals-n13">Secret Rotation</span>
            <span class="text-sm text-naturals-n13">{{ getComponentInRotation }}</span>
          </div>
          <div class="flex min-h-20 items-center gap-2 border-t-8 border-naturals-n4 p-4 text-xs">
            <TIcon
              v-if="clusterLocked || machineLockedForSecretRotation"
              icon="pause-circle"
              class="h-6 w-6"
            />
            <TIcon v-else icon="loading" class="h-6 w-6 animate-spin text-yellow-y1" />
            <div class="flex-1">
              {{ secretRotationStatus.spec.status }}
              <template v-if="clusterLocked">- waiting for cluster to be unlocked</template>
              <template v-else-if="secretRotationStatus.spec.status">
                - {{ secretRotationStatus.spec.step }}
              </template>
            </div>
          </div>
        </div>
        <div class="flex gap-5">
          <div class="mb-5 flex-1 rounded bg-naturals-n2 px-6 py-5">
            <div class="mb-3">
              <span class="text-sm text-naturals-n13">Features</span>
            </div>
            <div class="flex flex-col gap-2">
              <ClusterWorkloadProxyingCheckbox
                :model-value="enableWorkloadProxy"
                :disabled="!canManageClusterFeatures || !features?.spec.enable_workload_proxying"
                @update:model-value="(value) => setClusterWorkloadProxy(clusterId, value)"
              />
              <EmbeddedDiscoveryServiceCheckbox
                :model-value="useEmbeddedDiscoveryService"
                :disabled="!canManageClusterFeatures || !isEmbeddedDiscoveryServiceAvailable"
                @update:model-value="(value) => toggleUseEmbeddedDiscoveryService(value)"
              />
              <ClusterEtcdBackupCheckbox
                :backup-status="backupStatus"
                :cluster="currentCluster.spec"
                @update:cluster="(spec) => setClusterEtcdBackupsConfig(clusterId, spec)"
              />
            </div>
          </div>
          <div class="mb-5 flex-1 rounded bg-naturals-n2 px-6 py-5">
            <div class="mb-3">
              <span class="text-sm text-naturals-n13">Labels</span>
            </div>
            <ItemLabels
              :resource="currentCluster"
              :add-label-func="addClusterLabels"
              :remove-label-func="removeClusterLabels"
            />
          </div>
        </div>
        <div class="flex-col rounded bg-naturals-n2 pt-5">
          <div class="flex px-6 pb-4">
            <span class="text-sm text-naturals-n13">Machines</span>
          </div>
          <div class="grid grid-cols-[repeat(4,1fr)_--spacing(18)]">
            <ClusterMachines is-subgrid :cluster-i-d="clusterId" />
          </div>
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
