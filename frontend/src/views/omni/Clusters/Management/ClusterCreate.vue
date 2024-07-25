<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="flex gap-1 items-start">
      <page-header title="Create Cluster" class="flex-1"/>
    </div>
    <div class="flex flex-col items-stretch gap-4 flex-1">
      <div class="flex w-full gap-2 h-9">
        <t-input
          title="Cluster Name"
          class="flex-1 h-full"
          placeholder="..."
          :model-value="state.cluster.name ?? ''"
          @update:model-value="value => state.cluster.name = value"
        />
        <t-select-list
          class="h-full"
          title="Talos Version"
          :values="talosVersions"
          :defaultValue="state.cluster.talosVersion"
          @checkedValue="(value) => (state.cluster.talosVersion = value)"
        />
        <t-select-list
          class="h-full"
          title="Kubernetes Version"
          :values="kubernetesVersions"
          :defaultValue="state.cluster.kubernetesVersion"
          @checkedValue="(value) => (state.cluster.kubernetesVersion = value)"
          ref="kubernetesVersionSelector"
        />
        <t-button
          type="primary"
          :icon="hasConfigs ? 'settings-toggle' : 'settings'"
          @click="openPatchConfig"
        >
          Config Patches
        </t-button>
      </div>
      <div class="text-naturals-N13">Cluster Labels</div>
      <item-labels :resource="labelContainer" :add-label-func="addLabels" :remove-label-func="removeLabels"/>
      <div class="text-naturals-N13">Cluster Features</div>
      <div class="flex flex-col gap-3 max-w-sm">
        <tooltip placement="bottom">
          <template #description>
            <div class="flex flex-col gap-1 p-2">
              <p>Encrypt machine disks using Omni as a key management server.</p>
              <p>Once cluster is created it is not possible to update encryption settings.</p>
              <p class="text-primary-P2">This feature is only available for Talos >= 1.5.0.</p>
            </div>
          </template>
          <t-checkbox :checked="state.cluster.features?.encryptDisks" label="Encrypt Disks" @click="state.cluster.features.encryptDisks = !state.cluster.features.encryptDisks && supportsEncryption" :disabled="!supportsEncryption"/>
        </tooltip>
        <cluster-workload-proxying-checkbox
            :checked="state.cluster.features.enableWorkloadProxy"
            @click="() => (state.cluster.features.enableWorkloadProxy = !state.cluster.features.enableWorkloadProxy)"/>
        <embedded-discovery-service-checkbox
            :checked="state.cluster.features.useEmbeddedDiscoveryService"
            :disabled="!isEmbeddedDiscoveryServiceAvailable"
            :talos-version="state.cluster.talosVersion"
            @click="toggleUseEmbeddedDiscoveryService"/>
        <cluster-etcd-backup-checkbox :backup-status="backupStatus" @update:cluster="(spec) => {
          state.cluster.etcdBackupConfig = spec.backup_configuration
        }" :cluster="{
          backup_configuration: state.cluster.etcdBackupConfig
        }"/>
      </div>
      <div class="text-naturals-N13">Machine Sets</div>
      <MachineSets/>
      <div class="text-naturals-N13">Available Machines</div>
      <t-list
        :opts="[{
          resource: resource,
          runtime: Runtime.Omni,
          selectors: [
            `${MachineStatusLabelAvailable}`,
            `${MachineStatusLabelConnected}`,
            `!${MachineStatusLabelInvalidState}`,
            `${MachineStatusLabelReportingEvents}`,
          ],
          sortByField: 'created'
        },
        {
          resource: {
            type: MachineConfigGenOptionsType,
            namespace: DefaultNamespace,
          },
          runtime: Runtime.Omni,
        }]"
        search
        pagination
        class="flex-1"
        ref="list"
      >
        <template #norecords>
          <t-alert v-if="!$slots.norecords" type="info" title="No Machines Available"
            >Machine is available when it is connected, not allocated and is reporting Talos
            events.</t-alert
          >
        </template>
        <template #default="{ items, searchQuery }">
          <cluster-machine-item
            v-for="item in items"
            :version-mismatch="detectVersionMismatch(item)"
            :key="itemID(item)"
            :reset="reset"
            :item="item"
            :search-query="searchQuery"
            @filter-label="filterByLabel"
          />
        </template>
      </t-list>
      <div
        v-if="state.controlPlanesCount != 0"
        class="bg-naturals-N1 border-t border-naturals-N4 -mb-6 -mx-6 h-16 flex items-center py-3 px-5"
      >
        <cluster-menu
          class="w-full"
          :controlPlanes="state.controlPlanesCount"
          :workers="state.workersCount"
          :onSubmit="createCluster"
          :onReset="() => reset++"
          :disabled="!canCreateClusters"
          action="Create Cluster"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, onMounted, computed, watch } from "vue";
import { Runtime } from "@/api/common/omni.pb";
import {
  DefaultNamespace,
  MachineStatusType,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReportingEvents,
  MachineConfigGenOptionsType,
  TalosVersionType,
  DefaultKubernetesVersion,
PatchBaseWeightMachineSet,
PatchBaseWeightCluster,
} from "@/api/resources";
import { MachineStatusSpec, TalosVersionSpec } from "@/api/omni/specs/omni.pb";
import WatchResource, { itemID } from "@/api/watch";
import { showError, showSuccess } from "@/notification";
import { Resource } from "@/api/grpc";
import { clusterSync, nextAvailableClusterName, embeddedDiscoveryServiceAvailable } from "@/methods/cluster";
import { useRouter } from "vue-router";
import { showModal } from "@/modal";
import * as semver from "semver";
import yaml from "js-yaml";

import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TAlert from "@/components/TAlert.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import ClusterMachineItem from "@/views/omni/Clusters/Management/ClusterMachineItem.vue";
import ConfigPatchEdit from "@/views/omni/Modals/ConfigPatchEdit.vue";
import ClusterMenu from "@/views/omni/Clusters/ClusterMenu.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import TList from "@/components/common/List/TList.vue";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import { canCreateClusters } from "@/methods/auth";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import ClusterWorkloadProxyingCheckbox from "@/views/omni/Clusters/ClusterWorkloadProxyingCheckbox.vue";
import ClusterEtcdBackupCheckbox from "@/views/omni/Clusters/ClusterEtcdBackupCheckbox.vue";
import UntaintSingleNode from "@/views/omni/Modals/UntaintSingleNode.vue";
import MachineSets from "./MachineSets.vue";
import { initState, PatchID } from "@/states/cluster-management";
import { setupBackupStatus } from "@/methods";
import EmbeddedDiscoveryServiceCheckbox from "@/views/omni/Clusters/EmbeddedDiscoveryServiceCheckbox.vue";

const labelContainer: Ref<Resource> = computed(() => {
  return {
    metadata: {
        id: "label-container",
        labels: state.value.cluster.labels ?? {},
    },
    spec: {}
  };
});

const { status: backupStatus } = setupBackupStatus();

const state = initState();

const addLabels = (id: string, ...labels: string[]) => {
  state.value.addClusterLabels(labels);
}

const removeLabels = (id: string, ...keys: string[]) => {
  state.value.removeClusterLabels(keys);
}

const supportsEncryption = computed(() => {
  return semver.compare(state.value.cluster.talosVersion, "v1.5.0") >= 0;
});

const router = useRouter();

const kubernetesVersionSelector: Ref<{ selectItem: (s: string) => void } | undefined> = ref();

const talosVersionsList: Ref<Resource<TalosVersionSpec>[]> = ref([]);
const talosVersionsWatch = new WatchResource(talosVersionsList);
const reset = ref(0);

const kubernetesVersions: Ref<string[]> = computed(() => {
  for (const version of talosVersionsList.value) {
    if (version.spec.version == state.value.cluster.talosVersion) {
      return version.spec.compatible_kubernetes_versions ?? [];
    }
  }

  return [];
});

watch(kubernetesVersions, k8sVersions => {
  if (k8sVersions.length == 0) {
    kubernetesVersionSelector?.value?.selectItem("")
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
});

talosVersionsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
});

const isEmbeddedDiscoveryServiceAvailable = ref(false);

watch(state.value.cluster, async cluster => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(cluster?.talosVersion);

  if (!isEmbeddedDiscoveryServiceAvailable.value) {
    state.value.cluster.features.useEmbeddedDiscoveryService = false;
  }
});

const toggleUseEmbeddedDiscoveryService = async () => {
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(state.value.cluster?.talosVersion);

  if (!isEmbeddedDiscoveryServiceAvailable.value) {
    state.value.cluster.features.useEmbeddedDiscoveryService = false;

    return;
  }

  state.value.cluster.features.useEmbeddedDiscoveryService = !state.value.cluster.features.useEmbeddedDiscoveryService;
}

onMounted(async () => {
  state.value.cluster.name = await nextAvailableClusterName(state.value.cluster.name ?? "talos-default");
  isEmbeddedDiscoveryServiceAvailable.value = await embeddedDiscoveryServiceAvailable(state.value.cluster?.talosVersion);
});

const createCluster = async () => {
  if (state.value.untaintSingleNode()) {
    showModal(UntaintSingleNode, { onContinue: createCluster_ })
  } else {
    await createCluster_(false);
  }
};

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const clusterVersion = semver.parse(state.value.cluster.talosVersion);
  const machineVersion = semver.parse(machine.spec.talos_version);

  const installed = machine.spec.hardware?.blockdevices?.find(item => item.system_disk);

  if (!machineVersion || !clusterVersion) {
    return null;
  }

  if (!installed) {
    if (machineVersion?.major == clusterVersion?.major && machineVersion?.minor == clusterVersion?.minor) {
      return null;
    }

    return "The machine running from ISO or PXE must have the same major and minor version as the cluster it is going to be added to. Please use another ISO or change the cluster Talos version";
  }
  if (machineVersion?.major <= clusterVersion?.major && machineVersion?.minor <= clusterVersion?.minor) {
    return null;
  }

  return "The machine has newer Talos version installed: downgrade is not allowed. Upgrade the machine or change Talos cluster version";
}

const createCluster_ = async (untaint: boolean) => {
  if (typeof state.value.controlPlanesCount === 'number' && (state.value.controlPlanesCount - 1) % 2 !== 0) {
    showError(
      "Invalid Number of Control Planes",
      "The total number of control plane nodes must be an odd number to ensure etcd stability. (Three control plane nodes are required for a highly available control plane.)"
    )

    return;
  }

  if (untaint) {
    state.value.controlPlanes().patches[PatchID.Untaint] = {
      data: yaml.dump({
        cluster: {
          allowSchedulingOnControlPlanes: true
        }
      }),
      weight: PatchBaseWeightMachineSet,
      systemPatch: true,
    }
  }

  try {
    await clusterSync(state.value.resources());
  } catch (e) {
    if (e.message && e.message.indexOf("already exists") >= 0) {
      state.value.cluster.name = await nextAvailableClusterName("talos-default");
    }

    if (e.errorNotification) {
      showError(e.errorNotification.title, e.errorNotification.details);

      return;
    }

    showError("Failed to Create the Cluster", e.message);

    return;
  }

  showSuccess(
    "Succesfully Created Cluster",
    `Cluster name: ${state.value.cluster.name}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`
  );

  const clusterName = state.value.cluster.name;

  initState();

  router.push({ name: "ClusterOverview", params: { cluster: clusterName } });
}

const resource = {
  namespace: DefaultNamespace,
  type: MachineStatusType,
};

const talosVersions = computed(() => {
  const res: string[] = [];

  for (const version of talosVersionsList.value) {
    if (version.spec.deprecated) {
      continue;
    }

    res.push(version.spec.version!);
  }

  return res;
});

const hasConfigs = computed(() => {
  return Object.keys(state.value.cluster.patches).length > 0;
});

const openPatchConfig = () => {
  showModal(ConfigPatchEdit, {
    tabs: [
      {
        id: "Cluster",
        config: state.value.cluster.patches[PatchID.Default]?.data ?? "",
      },
    ],
    onSave: async (config: string) => {
      if (config == "") {
        delete state.value.cluster.patches[PatchID.Default];

        return;
      }

      state.value.cluster.patches[PatchID.Default] = {
        data: config,
        weight: PatchBaseWeightCluster,
      }
    },
  });
};

const list: Ref<{ addFilterLabel: (label: { key: string, value?: string }) => void } | null> = ref(null);

const filterByLabel = (e: { key: string, value?: string }) => {
  if (list.value) {
    list.value.addFilterLabel(e);
  }
}
</script>
