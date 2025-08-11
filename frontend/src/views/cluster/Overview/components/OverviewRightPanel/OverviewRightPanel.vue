<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import * as semver from 'semver'
import type { Ref } from 'vue'
import { computed, ref, toRefs } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterDiagnosticsSpec,
  ClusterDiagnosticsSpecNode,
  ClusterStatusSpec,
  ControlPlaneStatusSpec,
  EtcdBackupStatusSpec,
  KubernetesStatusSpec,
  KubernetesUpgradeStatusSpec,
  TalosUpgradeStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterDiagnosticsType,
  ClusterStatusType,
  ControlPlaneStatusType,
  DefaultNamespace,
  EtcdBackupStatusType,
  KubernetesStatusType,
  MetricsNamespace,
} from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import type { BackupsStatus } from '@/methods'
import { downloadKubeconfig, downloadTalosconfig } from '@/methods'
import { setupClusterPermissions } from '@/methods/auth'
import { triggerEtcdBackup } from '@/methods/cluster'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import { formatISO } from '@/methods/time'
import { showError } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import OverviewRightPanelCondition from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelCondition.vue'
import OverviewRightPanelItem from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue'
import TClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'

const route = useRoute()
const router = useRouter()

const currentCluster: Ref<Resource<ClusterStatusSpec> | undefined> = ref()
const currentClusterWatch = new Watch(currentCluster)

const backupStatus: Ref<Resource<EtcdBackupStatusSpec> | undefined> = ref()
const backupStatusWatch = new Watch(backupStatus)

const controlPlaneStatus: Ref<Resource<ControlPlaneStatusSpec> | undefined> = ref()
const controlPlaneStatusWatch = new Watch(controlPlaneStatus)

const kubernetesStatus: Ref<Resource<KubernetesStatusSpec> | undefined> = ref()
const kubernetesStatusWatch = new Watch(kubernetesStatus)

const clusterDiagnostics: Ref<Resource<ClusterDiagnosticsSpec> | undefined> = ref()
const clusterDiagnosticsWatch = new Watch(clusterDiagnostics)

const numNodesWithDiagnostics = computed(() => {
  return clusterDiagnostics.value?.spec.nodes?.length || 0
})

const numTotalDiagnostics = computed(() => {
  const nodes: ClusterDiagnosticsSpecNode[] = clusterDiagnostics.value?.spec?.nodes || []
  return nodes.reduce((sum, node) => sum + (node.num_diagnostics || 0), 0) || 0
})

const props = defineProps<{
  kubernetesUpgradeStatus: Resource<KubernetesUpgradeStatusSpec> | undefined
  talosUpgradeStatus: Resource<TalosUpgradeStatusSpec> | undefined
  etcdBackups: BackupsStatus | undefined
}>()

enum Update {
  Talos,
  Kubernetes,
}

const { kubernetesUpgradeStatus, talosUpgradeStatus } = toRefs(props)

currentClusterWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: route.params.cluster as string,
  },
})

controlPlaneStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ControlPlaneStatusType,
    id: controlPlaneMachineSetId(route.params.cluster as string),
  },
})

kubernetesStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesStatusType,
    id: route.params.cluster as string,
  },
})

backupStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: MetricsNamespace,
    type: EtcdBackupStatusType,
    id: route.params.cluster as string,
  },
})

clusterDiagnosticsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterDiagnosticsType,
    id: route.params.cluster as string,
  },
})

const lastBackupError = computed(() => {
  return backupStatus.value?.spec.error
})

const backupTime = computed(() => {
  const t = backupStatus?.value?.spec.last_backup_time

  if (!t) {
    return 'Never'
  }

  return formatISO(t, 'yyyy-LL-dd HH:mm:ss')
})

const startingEtcdBackup = ref(false)

const runEtcdBackup = async () => {
  startingEtcdBackup.value = true

  try {
    await triggerEtcdBackup(route.params.cluster as string)
  } catch (e) {
    showError('Failed to Trigger Manual Etcd Backup', e.message)
  }

  startingEtcdBackup.value = false
}

const kubernetesUpgradeAvailable = () => {
  return (kubernetesUpgradeStatus?.value?.spec?.upgrade_versions?.length ?? 0) > 0
}

const talosUpdateAvailable = () => {
  return (talosUpgradeStatus?.value?.spec?.upgrade_versions?.length ?? 0) > 0
}

const getUpgradeAvailable = (spec: {
  last_upgrade_version?: string
  current_upgrade_version?: string
  upgrade_versions?: string[]
}) => {
  if (spec.current_upgrade_version || !spec.upgrade_versions || !spec.last_upgrade_version) {
    return []
  }

  return spec.upgrade_versions.filter((version: string) => {
    return semver.compare(spec.last_upgrade_version!, version) === -1
  })
}

const kubernetesVersion = computed(() => {
  return getVersion(kubernetesUpgradeStatus.value!.spec)
})

const talosVersion = computed(() => {
  return getVersion(talosUpgradeStatus.value!.spec)
})

const newKubernetesVersionsAvailable = computed(() => {
  return getUpgradeAvailable(kubernetesUpgradeStatus.value!.spec)
})

const newTalosVersionsAvailable = computed(() => {
  return getUpgradeAvailable(talosUpgradeStatus.value!.spec)
})

const getVersion = (spec: { last_upgrade_version?: string; current_upgrade_version?: string }) => {
  if (spec.current_upgrade_version && spec.last_upgrade_version) {
    return `${spec.last_upgrade_version} ⇾ ${spec.current_upgrade_version}`
  }

  return spec.last_upgrade_version
}

const openClusterUpdate = (type: Update) => {
  if (type === Update.Kubernetes && !canUpdateKubernetes.value) {
    return
  }

  if (type === Update.Talos && !canUpdateTalos.value) {
    return
  }

  const modal = type === Update.Talos ? 'updateTalos' : 'updateKubernetes'

  router.push({ query: { modal: modal, cluster: currentCluster?.value?.metadata?.id } })
}

const openDownloadSupportBundle = () => {
  if (!canDownloadSupportBundle.value) {
    return
  }

  router.push({
    query: { modal: 'downloadSupportBundle', cluster: currentCluster?.value?.metadata?.id },
  })
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

<template>
  <div class="overview-right-box">
    <div class="overview-right-box-wrapper">
      <h3 class="overview-details-title">Cluster Details</h3>
      <ManagedByTemplatesWarning warning-style="short" />
      <OverviewRightPanelItem
        v-if="currentCluster?.metadata?.created"
        name="Created"
        :value="formatISO(currentCluster?.metadata?.created as string, 'yyyy-LL-dd HH:mm:ss')"
      />
      <OverviewRightPanelItem v-if="currentCluster" name="Status">
        <TClusterStatus :cluster="currentCluster" />
      </OverviewRightPanelItem>
      <OverviewRightPanelItem
        v-if="currentCluster?.spec?.machines?.total"
        name="Machines Healthy"
        :value="`${currentCluster?.spec?.machines?.healthy ?? 0}/${currentCluster?.spec?.machines?.total}`"
      />
      <OverviewRightPanelItem v-if="numNodesWithDiagnostics > 0" name="Node Warnings">
        <div class="flex items-center gap-1">
          {{ numTotalDiagnostics }} (in {{ numNodesWithDiagnostics }} nodes)
          <Tooltip
            description="Some machines have diagnostic warnings. See the machines section for details."
          >
            <TIcon class="h-4 w-4 text-yellow-y1" icon="warning" />
          </Tooltip>
        </div>
      </OverviewRightPanelItem>
      <OverviewRightPanelItem
        v-if="talosUpgradeStatus"
        name="Talos Version"
        @click="openClusterUpdate(Update.Talos)"
      >
        <div>{{ talosVersion }}</div>
        <Tooltip
          v-if="newTalosVersionsAvailable?.length"
          :description="`Newer Talos versions are avalable: ${newTalosVersionsAvailable.join(', ')}`"
        >
          <TIcon
            v-if="newTalosVersionsAvailable"
            class="h-4 w-4 cursor-pointer"
            icon="upgrade-available"
          />
        </Tooltip>
      </OverviewRightPanelItem>
      <OverviewRightPanelItem
        v-if="kubernetesUpgradeStatus"
        name="Kubernetes Version"
        @click="openClusterUpdate(Update.Kubernetes)"
      >
        <div>{{ kubernetesVersion }}</div>
        <Tooltip
          v-if="newKubernetesVersionsAvailable?.length"
          :description="`Newer Kubernetes versions are avalable: ${newKubernetesVersionsAvailable.join(', ')}`"
        >
          <TIcon class="h-4 w-4 cursor-pointer" icon="upgrade-available" />
        </Tooltip>
      </OverviewRightPanelItem>
    </div>
    <template v-if="currentCluster?.spec">
      <div class="divider" />
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <h3 class="overview-details-title">Control Plane</h3>
        <OverviewRightPanelItem name="Ready">
          <span :class="currentCluster?.spec?.controlplaneReady ? '' : 'text-red-r1'">
            {{ currentCluster?.spec?.controlplaneReady ? 'Yes' : 'No' }}
          </span>
        </OverviewRightPanelItem>
        <OverviewRightPanelItem v-if="currentCluster?.metadata?.created" name="Last Backup">
          <div v-if="startingEtcdBackup" class="flex gap-1">
            Starting...
            <TSpinner class="h-4 w-4" />
          </div>
          <template v-else>
            <Tooltip v-if="lastBackupError">
              <template #description>
                <div class="max-w-lg break-words">{{ lastBackupError }}</div>
              </template>
              <TIcon class="h-4 w-4 text-yellow-y1" icon="warning" />
            </Tooltip>
            {{ backupTime }}
            <Tooltip v-if="etcdBackups?.enabled" description="Trigger Etcd Backup">
              <TIcon
                class="h-4 w-4 cursor-pointer text-green-g1"
                icon="play-circle"
                @click="runEtcdBackup"
              />
            </Tooltip>
          </template>
        </OverviewRightPanelItem>
        <OverviewRightPanelCondition
          v-for="condition in controlPlaneStatus?.spec.conditions"
          :key="condition.type?.toString()"
          :condition="condition"
        />
      </div>
      <div class="divider" />
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <h3 class="overview-details-title">Kubernetes</h3>
        <OverviewRightPanelItem
          name="API Available"
          :value="currentCluster?.spec.kubernetesAPIReady ? 'Yes' : 'No'"
        />
        <OverviewRightPanelItem
          v-if="kubernetesStatus && kubernetesStatus.spec.nodes"
          name="Nodes"
          :value="kubernetesStatus.spec.nodes.length ?? 0"
        />
      </div>
      <div class="divider" />
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <div class="overview-details-item">
          <TButton
            :disabled="!canDownloadKubeconfig"
            class="overview-item-button w-full"
            type="primary"
            icon="kube-config"
            icon-position="left"
            @click="() => downloadKubeconfig(currentCluster!.metadata.id!)"
            >Download <code>kubeconfig</code></TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            :disabled="!canDownloadTalosconfig"
            class="overview-item-button w-full"
            type="primary"
            icon="talos-config"
            icon-position="left"
            @click="() => downloadTalosconfig(currentCluster!.metadata.id!)"
            >Download <code>talosconfig</code></TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            :disabled="!canDownloadSupportBundle"
            class="overview-item-button w-full"
            type="primary"
            icon="lifebuoy"
            icon-position="left"
            @click="openDownloadSupportBundle"
            >Download Support Bundle</TButton
          >
        </div>
      </div>
      <div class="divider" />
      <div class="overview-right-box-wrapper overview-right-box-wrapper-moved">
        <div class="overview-details-item">
          <TButton
            class="overview-item-button w-full"
            type="highlighted"
            icon="nodes"
            icon-position="left"
            :disabled="!canAddClusterMachines"
            @click="
              () =>
                $router.push({
                  name: 'ClusterScale',
                  params: { cluster: currentCluster?.metadata?.id },
                })
            "
            >Cluster Scaling</TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            class="overview-item-button w-full"
            type="primary"
            icon="kubernetes"
            icon-position="left"
            :disabled="!canUpdateKubernetes || !kubernetesUpgradeAvailable()"
            @click="openClusterUpdate(Update.Kubernetes)"
            >Update Kubernetes</TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            class="overview-item-button w-full"
            type="primary"
            icon="sidero-monochrome"
            icon-position="left"
            :disabled="!canUpdateTalos || !talosUpdateAvailable()"
            @click="openClusterUpdate(Update.Talos)"
            >Update Talos</TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            class="overview-item-button w-full"
            type="primary"
            icon="settings"
            icon-position="left"
            @click="
              () =>
                $router.push({
                  name: 'ClusterConfigPatches',
                  params: { cluster: currentCluster?.metadata?.id },
                })
            "
            >Config Patches</TButton
          >
        </div>
        <div class="overview-details-item">
          <TButton
            class="overview-item-button-red w-full"
            type="secondary"
            icon="delete"
            icon-position="left"
            :disabled="!canRemoveClusterMachines"
            @click="
              () =>
                $router.push({
                  query: { modal: 'clusterDestroy', cluster: currentCluster?.metadata?.id },
                })
            "
            >Destroy Cluster</TButton
          >
        </div>
      </div>
    </template>
    <div v-else class="flex items-center justify-center p-4">
      <TSpinner class="h-6 w-6" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../../index.css";

.divider {
  @apply my-3 w-full bg-naturals-n4;
  height: 1px;
}

.overview-right-box {
  @apply mb-4 h-auto w-full rounded bg-naturals-n2 py-5;
  max-width: 20%;
  min-width: 270px;
}

.overview-right-box-wrapper {
  @apply flex flex-col gap-4 px-2 lg:px-6;
}

.overview-details-title {
  @apply text-sm text-naturals-n13;
}

.overview-item-button-red {
  @apply text-red-r1;
}

.overview-details-item {
  @apply flex items-center justify-between;
}
</style>
