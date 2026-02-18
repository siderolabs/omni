<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { dump } from 'js-yaml'
import * as semver from 'semver'
import type { Ref } from 'vue'
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultKubernetesVersion,
  DefaultNamespace,
  LabelNoManualAllocation,
  MachineConfigGenOptionsType,
  MachineStatusLabelAvailable,
  MachineStatusLabelInstalled,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
  PatchBaseWeightCluster,
  PatchBaseWeightMachineSet,
  TalosVersionType,
} from '@/api/resources'
import WatchResource, { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import TAlert from '@/components/TAlert.vue'
import { setupBackupStatus } from '@/methods'
import { canCreateClusters } from '@/methods/auth'
import {
  ClusterCommandError,
  clusterSync,
  embeddedDiscoveryServiceAvailable,
  nextAvailableClusterName,
} from '@/methods/cluster'
import { showModal } from '@/modal'
import { showError, showSuccess } from '@/notification'
import { initState, PatchID } from '@/states/cluster-management'
import ClusterEtcdBackupCheckbox from '@/views/omni/Clusters/ClusterEtcdBackupCheckbox.vue'
import ClusterMenu from '@/views/omni/Clusters/ClusterMenu.vue'
import ClusterWorkloadProxyingCheckbox from '@/views/omni/Clusters/ClusterWorkloadProxyingCheckbox.vue'
import EmbeddedDiscoveryServiceCheckbox from '@/views/omni/Clusters/EmbeddedDiscoveryServiceCheckbox.vue'
import ClusterMachineItem from '@/views/omni/Clusters/Management/ClusterMachineItem.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import ConfigPatchEdit from '@/views/omni/Modals/ConfigPatchEdit.vue'
import UntaintSingleNode from '@/views/omni/Modals/UntaintSingleNode.vue'

import MachineSets from './MachineSets.vue'

const labelContainer: Ref<Resource> = computed(() => {
  return {
    metadata: {
      id: 'label-container',
      labels: state.value.cluster.labels ?? {},
    },
    spec: {},
  }
})

const { status: backupStatus } = setupBackupStatus()

const state = initState()

const addLabels = (_: string, ...labels: string[]) => {
  state.value.addClusterLabels(labels)
}

const removeLabels = (_: string, ...keys: string[]) => {
  state.value.removeClusterLabels(keys)
}

const supportsEncryption = computed(() => {
  return semver.compare(state.value.cluster.talosVersion ?? '', 'v1.5.0') >= 0
})

const router = useRouter()

const kubernetesVersionSelector = useTemplateRef('kubernetesVersionSelector')

const talosVersionsList: Ref<Resource<TalosVersionSpec>[]> = ref([])
const talosVersionsWatch = new WatchResource(talosVersionsList)
const reset = ref(0)

const kubernetesVersions: Ref<string[]> = computed(() => {
  for (const version of talosVersionsList.value) {
    if (version.spec.version === state.value.cluster.talosVersion) {
      return version.spec.compatible_kubernetes_versions ?? []
    }
  }

  return []
})

watch(kubernetesVersions, (k8sVersions) => {
  if (k8sVersions.length === 0) {
    kubernetesVersionSelector?.value?.selectItem('')
    return
  }

  const k8sVersionSet = new Set(k8sVersions)

  if (!state.value.cluster.kubernetesVersion) {
    kubernetesVersionSelector?.value?.selectItem(DefaultKubernetesVersion)

    return
  }

  // If currently selected Kubernetes version is not supported by the chosen Talos version
  if (!k8sVersionSet.has(state.value.cluster.kubernetesVersion)) {
    // if the default Kubernetes version is supported by the chosen Talos version, select it
    if (k8sVersionSet.has(DefaultKubernetesVersion)) {
      kubernetesVersionSelector?.value?.selectItem(DefaultKubernetesVersion)
      return
    }

    // select the latest supported Kubernetes version by the chosen Talos version (k8sVersions are sorted on backend)
    kubernetesVersionSelector?.value?.selectItem(k8sVersions[k8sVersions.length - 1])
  }
})

talosVersionsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
})

const isEmbeddedDiscoveryServiceAvailable = ref(false)

watch(state.value.cluster, async (cluster) => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(
    cluster?.talosVersion,
  )

  if (!isEmbeddedDiscoveryServiceAvailable.value) {
    state.value.cluster.features.useEmbeddedDiscoveryService = false
  }
})

const toggleUseEmbeddedDiscoveryService = async () => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(
    state.value.cluster?.talosVersion,
  )

  if (!isEmbeddedDiscoveryServiceAvailable.value) {
    state.value.cluster.features.useEmbeddedDiscoveryService = false

    return
  }

  state.value.cluster.features.useEmbeddedDiscoveryService =
    !state.value.cluster.features.useEmbeddedDiscoveryService
}

onMounted(async () => {
  state.value.cluster.name = await nextAvailableClusterName(
    state.value.cluster.name ?? 'talos-default',
  )
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(
    state.value.cluster?.talosVersion,
  )
})

const createCluster = async () => {
  if (state.value.untaintSingleNode()) {
    showModal(UntaintSingleNode, { onContinue: createCluster_ })
  } else {
    await createCluster_(false)
  }
}

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const clusterVersion = semver.parse(state.value.cluster.talosVersion)
  const machineVersion = semver.parse(machine.spec.talos_version)

  const installed = machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined
  const inAgentMode = !!machine.spec.schematic?.in_agent_mode

  if (!machineVersion || !clusterVersion) {
    return null
  }

  if (!installed) {
    if (
      machineVersion?.major === clusterVersion?.major &&
      machineVersion?.minor === clusterVersion?.minor
    ) {
      return null
    }

    if (inAgentMode) {
      return null
    }

    return 'The machine running from ISO or PXE must have the same major and minor version as the cluster it is going to be added to. Please use another ISO or change the cluster Talos version'
  }
  if (
    machineVersion?.major <= clusterVersion?.major &&
    machineVersion?.minor <= clusterVersion?.minor
  ) {
    return null
  }

  return 'The machine has newer Talos version installed: downgrade is not allowed. Upgrade the machine or change Talos cluster version'
}

const createCluster_ = async (untaint: boolean) => {
  if (
    typeof state.value.controlPlanesCount === 'number' &&
    (state.value.controlPlanesCount - 1) % 2 !== 0
  ) {
    showError(
      'Invalid Number of Control Planes',
      'The total number of control plane nodes must be an odd number to ensure etcd stability. (Three control plane nodes are required for a highly available control plane.)',
    )

    return
  }

  if (untaint) {
    state.value.controlPlanes().patches[PatchID.Untaint] = {
      data: dump({
        cluster: {
          allowSchedulingOnControlPlanes: true,
        },
      }),
      weight: PatchBaseWeightMachineSet,
      systemPatch: true,
    }
  }

  try {
    await clusterSync(state.value.resources())
  } catch (e) {
    if (e.message && e.message.indexOf('already exists') >= 0) {
      state.value.cluster.name = await nextAvailableClusterName('talos-default')
    }

    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Create the Cluster', e.message)

    return
  }

  showSuccess(
    'Succesfully Created Cluster',
    `Cluster name: ${state.value.cluster.name}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`,
  )

  const clusterName = state.value.cluster.name

  initState()

  router.push({ name: 'ClusterOverview', params: { cluster: clusterName } })
}

const resource = {
  namespace: DefaultNamespace,
  type: MachineStatusType,
}

const talosVersions = computed(() => {
  const res: string[] = []

  for (const version of talosVersionsList.value) {
    if (version.spec.deprecated) {
      continue
    }

    res.push(version.spec.version!)
  }

  res.sort(semver.compare)

  return res
})

const hasConfigs = computed(() => {
  return Object.keys(state.value.cluster.patches).length > 0
})

const openPatchConfig = () => {
  showModal(ConfigPatchEdit, {
    talosVersion: state.value.cluster.talosVersion,
    id: 'Cluster',
    config: state.value.cluster.patches[PatchID.Default]?.data ?? '',
    onSave: async (config: string) => {
      if (config === '') {
        delete state.value.cluster.patches[PatchID.Default]

        return
      }

      state.value.cluster.patches[PatchID.Default] = {
        data: config,
        weight: PatchBaseWeightCluster,
      }
    },
  })
}

const list = useTemplateRef('list')
</script>

<template>
  <div class="flex flex-col">
    <div class="flex items-start gap-1">
      <PageHeader title="Create Cluster" class="flex-1" />
    </div>
    <div class="flex flex-1 flex-col items-stretch gap-4">
      <div class="flex h-9 w-full gap-2">
        <TInput
          title="Cluster Name"
          class="h-full flex-1"
          placeholder="..."
          :model-value="state.cluster.name ?? ''"
          @update:model-value="(value) => (state.cluster.name = value)"
        />
        <TSelectList
          class="h-full"
          title="Talos Version"
          :values="talosVersions"
          :default-value="state.cluster.talosVersion"
          @checked-value="(value) => (state.cluster.talosVersion = value)"
        />
        <TSelectList
          ref="kubernetesVersionSelector"
          class="h-full"
          title="Kubernetes Version"
          :values="kubernetesVersions"
          :default-value="state.cluster.kubernetesVersion"
          @checked-value="(value) => (state.cluster.kubernetesVersion = value)"
        />
        <TButton
          variant="primary"
          :icon="hasConfigs ? 'settings-toggle' : 'settings'"
          @click="openPatchConfig"
        >
          Config Patches
        </TButton>
      </div>
      <div class="text-naturals-n13">Cluster Labels</div>
      <ItemLabels
        :resource="labelContainer"
        :add-label-func="addLabels"
        :remove-label-func="removeLabels"
      />
      <div class="text-naturals-n13">Cluster Features</div>
      <div class="flex max-w-sm flex-col gap-3">
        <Tooltip placement="bottom">
          <template #description>
            <div class="flex flex-col gap-1 p-2">
              <p>Encrypt machine disks using Omni as a key management server.</p>
              <p>Once cluster is created it is not possible to update encryption settings.</p>
            </div>
          </template>
          <TCheckbox
            v-model="state.cluster.features.encryptDisks"
            label="Encrypt Disks"
            :disabled="!supportsEncryption"
          />
        </Tooltip>
        <ClusterWorkloadProxyingCheckbox v-model="state.cluster.features.enableWorkloadProxy" />
        <EmbeddedDiscoveryServiceCheckbox
          :checked="state.cluster.features.useEmbeddedDiscoveryService"
          :disabled="!isEmbeddedDiscoveryServiceAvailable"
          :talos-version="state.cluster.talosVersion"
          @click="toggleUseEmbeddedDiscoveryService"
        />
        <ClusterEtcdBackupCheckbox
          :backup-status="backupStatus"
          :cluster="{
            backup_configuration: state.cluster.etcdBackupConfig,
          }"
          @update:cluster="
            (spec) => {
              state.cluster.etcdBackupConfig = spec.backup_configuration
            }
          "
        />
      </div>
      <div class="text-naturals-n13">Machine Sets</div>
      <MachineSets />
      <div class="text-naturals-n13">Available Machines</div>
      <TList
        ref="list"
        :opts="[
          {
            resource: resource,
            runtime: Runtime.Omni,
            selectors: [
              `${MachineStatusLabelAvailable}`,
              `${MachineStatusLabelReadyToUse}`,
              `!${MachineStatusLabelInvalidState}`,
              `${MachineStatusLabelReportingEvents}`,
              `!${LabelNoManualAllocation}`,
            ],
            sortByField: 'created',
          },
          {
            resource: {
              type: MachineConfigGenOptionsType,
              namespace: DefaultNamespace,
            },
            runtime: Runtime.Omni,
          },
        ]"
        search
        pagination
        class="flex-1"
      >
        <template #norecords>
          <TAlert v-if="!$slots.norecords" type="info" title="No Machines Available">
            Machine is available when it is connected, not allocated and is reporting Talos events.
          </TAlert>
        </template>
        <template #default="{ items, searchQuery }">
          <ClusterMachineItem
            v-for="item in items"
            :key="itemID(item)"
            :version-mismatch="detectVersionMismatch(item)"
            :reset="reset"
            :item="item"
            :search-query="searchQuery"
            @filter-label="list?.addFilterLabel"
          />
        </template>
      </TList>
      <div
        v-if="state.controlPlanesCount !== 0"
        class="-mx-6 -mb-6 flex h-16 items-center border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
      >
        <ClusterMenu
          class="w-full"
          :control-planes="state.controlPlanesCount"
          :workers="state.workersCount"
          :on-submit="createCluster"
          :on-reset="() => reset++"
          :disabled="!canCreateClusters"
          action="Create Cluster"
        />
      </div>
    </div>
  </div>
</template>
