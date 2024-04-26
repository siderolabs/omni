<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-3">
    <page-header :title="`Add Machines to Cluster ${$route.params.cluster}`"/>
    <managed-by-templates-warning :cluster="currentCluster"/>
    <template v-if="existingResources.length > 0">
      <div class="text-naturals-N13">Machine Sets</div>
      <machine-sets/>
      <div class="text-naturals-N13">Available Machines</div>
      <watch
        class="flex-1"
        :opts="[{
          resource: resource,
          selectors: [
            `${MachineStatusLabelAvailable}`,
            `${MachineStatusLabelConnected}`,
            `!${MachineStatusLabelInvalidState}`,
            `${MachineStatusLabelReportingEvents}`
          ],
          runtime: Runtime.Omni,
          sortByField: 'created',
        },
        {
          resource: {
            type: MachineConfigGenOptionsType,
            namespace: DefaultNamespace,
          },
          runtime: Runtime.Omni,
        }]"
        errorsAlert
        noRecordsAlert
        spinner
      >
        <template #norecords>
          <t-alert
            v-if="!$slots.norecords"
            type="info"
            title="No Machines Available"
            >Machine is available when it is connected, not allocated and is reporting Talos events.</t-alert
          >
        </template>
        <template #default="{ items }">
          <cluster-machine-item
            v-for="item in items"
            :key="itemID(item)"
            :item="item"
            :version-mismatch="detectVersionMismatch(item)"
          />
        </template>
      </watch>
      <div
        class="bg-naturals-N1 border-t border-naturals-N4 -mb-6 -mx-6 h-16 flex items-center py-3 px-5">
        <cluster-menu
          class="w-full"
          :controlPlanes="state.controlPlanesCount"
          :workers="state.workersCount"
          :onSubmit="scaleCluster"
          action="Update"
          :warning="quorumWarning"
        />
      </div>
    </template>
    <div v-else class="flex-1 flex items-center justify-center">
      <t-spinner class="w-6 h-6"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import {
  DefaultNamespace,
  MachineStatusType,
  MachineConfigGenOptionsType,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReportingEvents,
} from "@/api/resources";
import { showError, showSuccess } from "@/notification";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { Resource, ResourceTyped } from "@/api/grpc";
import { itemID } from "@/api/watch";
import { clusterSync } from "@/methods/cluster";
import pluralize from "pluralize";
import { populateExisting, state } from "@/states/cluster-management";

import TAlert from "@/components/TAlert.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import ClusterMachineItem from "@/views/omni/Clusters/Management/ClusterMachineItem.vue";
import ClusterMenu from "@/views/omni/Clusters/ClusterMenu.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import MachineSets from "@/views/omni/Clusters/Management/MachineSets.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";
import { ClusterSpec, MachineStatusSpec } from "@/api/omni/specs/omni.pb";

import * as semver from "semver";

type Props = {
  currentCluster: ResourceTyped<ClusterSpec>,
};

defineProps<Props>();

const route = useRoute();
const router = useRouter();

const quorumWarning = computed(() => {
  if (typeof state.value.controlPlanesCount === 'string') {
    return undefined;
  }

  const totalMachines = state.value.controlPlanesCount as number;

  if ((totalMachines + 1) % 2 === 0) {
    return undefined;
  }

  return `${pluralize("Control Plane", totalMachines, true)} will not provide fault-tolerance with etcd quorum requirements. The total number of control plane machines must be an odd number to ensure etcd stability. Please add one more machine or remove one.`;
});

const clusterName = route.params.cluster;

const scaleCluster = async () => {
  try {
    await clusterSync(state.value.resources(), existingResources.value);
  } catch (e) {
    if (e.errorNotification) {
      showError(
        e.errorNotification.title,
        e.errorNotification.details,
      )

      return;
    }

    showError("Failed to Scale the Cluster", e.message);

    return;
  }

  await router.push({ name: 'ClusterOverview', params: { cluster: clusterName as string } });

  showSuccess(
    "Updated Cluster Configuration",
    `Cluster name: ${clusterName}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`
  );
};

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const clusterVersion = semver.parse(state.value.cluster.talosVersion);
  const machineVersion = semver.parse(machine.spec.talos_version);

  const installed = machine.spec.hardware?.blockdevices?.find(item => item.system_disk);

  if (!installed) {
    if (machineVersion.major == clusterVersion.major && machineVersion.minor == clusterVersion.minor) {
      return null;
    }

    return "The machine running from ISO or PXE must have the same major and minor version as the cluster it is going to be added to. Please use another ISO or change the cluster Talos version";
  }

  if (machineVersion.major <= clusterVersion.major && machineVersion.minor <= clusterVersion.minor) {
    return null;
  }

  return "The machine has newer Talos version installed: downgrade is not allowed. Upgrade the machine or change Talos cluster version";
}

const resource = {
  namespace: DefaultNamespace,
  type: MachineStatusType,
};

const existingResources = ref<Resource[]>([]);
onMounted(async () => {
  existingResources.value = await populateExisting(clusterName as string);
});
</script>
