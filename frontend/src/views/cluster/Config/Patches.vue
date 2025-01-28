<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-4 overflow-y-auto">
    <managed-by-templates-warning :cluster="cluster"/>
    <div class="flex gap-4">
      <t-input
        class="flex-1"
        placeholder="Search..."
        icon="search"
        v-model="filter"
      />
      <t-button type="highlighted" @click="openPatchCreate" :disabled="!canManageConfigPatches">Create Patch</t-button>
    </div>
    <div class="flex-1 font-sm">
      <div v-if="loading" class="w-full h-full flex items-center justify-center">
        <t-spinner class="w-6 h-6"/>
      </div>
      <t-alert v-else-if="patches.length === 0"
        title="No Config Patches"
        type="info"
      >
        There are no config patches {{
          machine?.metadata.id ? `associated with machine ${machine.metadata.id}` :
          cluster?.metadata.id ? `associated with cluster ${cluster.metadata.id}` : `on the account`}}
      </t-alert>
      <disclosure v-else as="div" :defaultOpen="true" v-for="group in routes" :key="group.name">
        <template v-slot="{ open }">
          <disclosure-button as="div" class="disclosure">
            <t-icon icon="arrow-up" class="w-4 h-4 absolute top-0 bottom-0 m-auto right-4" :class="{'rotate-180': open}"/>
            <div class="grid grid-cols-4">
              <word-highlighter :text-to-highlight="group.name" :query="filter" highlight-class="bg-naturals-N14"/>
              <div>ID</div>
              <div class="col-span-2">Description</div>
            </div>
          </disclosure-button>
          <disclosure-panel>
            <div v-for="item in group.items" :key="item.name"
            @click="() => $router.push(item.route)"
            class="grid grid-cols-4 relative items-center gap-2 w-full text-xs px-4 py-2 my-2 cursor-pointer hover:text-naturals-N12 hover:bg-naturals-N3 transition-colors duration-200">
              <icon-button @click.stop="() => { $router.push({ query: { modal: 'configPatchDestroy', id: item.id } }); }" :disabled="!canManageConfigPatches" icon="delete" class="w-4 h-4 absolute top-0 bottom-0 m-auto right-3"/>
              <div class="pointer-events-none flex items-center gap-4">
                <document-icon class="w-4 h-4"/>
                <word-highlighter :text-to-highlight="item.name" :query="filter" highlight-class="bg-naturals-N14"/>
              </div>
              <word-highlighter :text-to-highlight="item.id" :query="filter" highlight-class="pointer-events-none bg-naturals-N14"/>
              <div class="pointer-events-none col-span-2 truncate">
                {{ item.description }}
              </div>
            </div>
          </disclosure-panel>
        </template>
      </disclosure>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  ConfigPatchType,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelMachineSet,
  LabelClusterMachine,
  LabelMachine,
  LabelHostname,
  VirtualNamespace,
  ClusterPermissionsType,
  ConfigPatchDescription,
  ConfigPatchName,
} from "@/api/resources";
import { Resource, ResourceService } from "@/api/grpc";
import { RouteLocationRaw, useRoute, useRouter } from "vue-router";
import { ref, computed, Ref, watch, onMounted, toRefs } from "vue";
import { ClusterSpec, ConfigPatchSpec } from "@/api/omni/specs/omni.pb";
import Watch, { WatchOptions } from "@/api/watch";
import { v4 as uuidv4 } from "uuid";

import TAlert from "@/components/TAlert.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import { Disclosure, DisclosureButton, DisclosurePanel } from "@headlessui/vue";
import TInput from "@/components/common/TInput/TInput.vue";
import { DocumentIcon } from "@heroicons/vue/24/solid";
import WordHighlighter from "vue-word-highlighter";
import IconButton from "@/components/common/Button/IconButton.vue";
import { Runtime } from "@/api/common/omni.pb";
import { LabelSystemPatch } from "@/api/resources";
import { withRuntime } from "@/api/options";
import { canManageMachineConfigPatches, canReadMachineConfigPatches } from "@/methods/auth";
import { controlPlaneTitle, machineSetTitle, defaultWorkersTitle, workersTitlePrefix } from "@/methods/machineset";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";
import TButton from "@/components/common/Button/TButton.vue";

const route = useRoute();
const router = useRouter();
const filter = ref("");
const machineStatuses: Ref<Resource[]> = ref([]);
const machineStatusesWatch = new Watch(machineStatuses);

type Props = {
  cluster?: Resource<ClusterSpec>
  machine?: Resource,
};

const props = defineProps<Props>();

const { machine, cluster } = toRefs(props);

const selectors = computed<string[] | void>(() => {
  const res: string[] = [];

  if (cluster.value) {
    res.push(`${LabelCluster}=${cluster.value.metadata.id}`);
  } else if (machine.value) {
    res.push(
      `${LabelMachine}=${machine.value.metadata.id}`,
      `${LabelClusterMachine}=${machine.value.metadata.id}`
    );
  } else {
    return;
  }

  return res.map(item => item + `,!${LabelSystemPatch}`);
});

const patches: Ref<Resource<ConfigPatchSpec>[]> = ref([]);
const patchesWatch = new Watch(patches);
const loading = computed(() => {
  return patchesWatch.loading.value || machineStatusesWatch.loading.value;
});

patchesWatch.setup(computed<WatchOptions | undefined>(() => {
  if (!selectors.value) {
    return;
  }

  return {
    runtime: Runtime.Omni,
    resource: { type: ConfigPatchType, namespace: DefaultNamespace },
    selectors: selectors.value,
    selectUsingOR: true,
  }
}));

machineStatusesWatch.setup({
    runtime: Runtime.Omni,
    resource: { type: ClusterMachineStatusType, namespace: DefaultNamespace }
});

const includes = (filter: string, values: string[]) => {
  for (const value of values) {
    if (!value) {
      continue;
    }

    if (value.indexOf(filter) !== -1) {
      return true;
    }
  }

  return false;
}

type item = {name: string, route: RouteLocationRaw, id: string, description?: string};

const patchTypeCluster = "Cluster";

const routes = computed(() => {
  const hostnames: Record<string, string> = {};

  machineStatuses.value.forEach((item: Resource) => {
    hostnames[item.metadata.id!] = item.metadata.labels![LabelHostname];
  });

  const groups: Record<string, item[]> = {};

  const addToGroup = (name: string, r: item) => {
    groups[name] = (groups[name] ?? []).concat([r]);
  }

  patches.value.forEach((item: Resource) => {
    const searchValues: string[] = [
      item.metadata.id!,
      (item.metadata.annotations || {})[ConfigPatchName],
      hostnames[(item.metadata.labels || {})[LabelClusterMachine]],
    ];

    if (filter.value != "" && !includes(filter.value, searchValues)) {
      return;
    }

    const patchEditPage = route.params.cluster ? "ClusterMachinePatchEdit" : "MachinePatchEdit";

    const r = {
      name: (item.metadata.annotations || {})[ConfigPatchName] || item.metadata.id!,
      icon: "document",
      route: {
        name: machine?.value ? patchEditPage : "ClusterPatchEdit",
        params: { patch: item.metadata.id! }
      },
      id: item.metadata.id!,
      description: item.metadata.annotations?.[ConfigPatchDescription],
    };

    const labels = item.metadata.labels || {};
    const machineID = labels[LabelMachine];
    if (machineID) {
      addToGroup(`Machine: ${machineID}`, r);
    } else if (labels[LabelClusterMachine]) {
      const id = labels[LabelClusterMachine];
      addToGroup(`Cluster Machine: ${hostnames[id] || id}`, r);
    } else if (labels[LabelMachineSet]) {
      const id = labels[LabelMachineSet];

      const title = machineSetTitle(route.params.cluster as string, id);

      addToGroup(`${title}`, r);
    } else if (labels[LabelCluster]) {
      addToGroup(`${patchTypeCluster}: ${labels[LabelCluster]}`, r);
    }
  });

  const result: {name: string, items: item[]}[] = [];
  for (const key in groups) {
    result.push({name: key, items: groups[key]});
  }

  const clusterPrefix = `${patchTypeCluster}: `;

  const categoryIndex = (name: string) => {
    if (name.startsWith(clusterPrefix)) {
      return 0;
    }

    if (name === controlPlaneTitle) {
      return 1;
    }

    if (name === defaultWorkersTitle) {
      return 2;
    }

    if (name.startsWith(workersTitlePrefix)) {
      return 3;
    }

    return 4;
  }

  result.sort((a, b): number => {
    const categoryA = categoryIndex(a.name);
    const categoryB = categoryIndex(b.name);

    if (categoryA !== categoryB) {
      return categoryA - categoryB;
    }

    return a.name.localeCompare(b.name);
  });

  return result;
});

const canReadConfigPatches = ref(false);
const canManageConfigPatches = ref(false);

const updatePermissions = async () => {
  if (cluster?.value) {
    const clusterPermissions = await ResourceService.Get({
      namespace: VirtualNamespace,
      type: ClusterPermissionsType,
      id: cluster?.value.metadata.id,
    }, withRuntime(Runtime.Omni))

    canReadConfigPatches.value = clusterPermissions?.spec?.can_read_config_patches || false;
    canManageConfigPatches.value = clusterPermissions?.spec?.can_manage_config_patches || false;
  } else if (machine?.value) {
    canReadConfigPatches.value = canReadMachineConfigPatches.value;
    canManageConfigPatches.value = canManageMachineConfigPatches.value;
  }
};

const openPatchCreate = () => {
  if (!cluster.value && !machine.value) {
    return;
  }

  router.push({
    name: cluster.value ? "ClusterPatchEdit" : "MachinePatchEdit",
    params: {
      patch: `500-${uuidv4()}`,
    },
  })
};

watch([() => machine.value, () => cluster.value],  async () => {
  await updatePermissions();
});

onMounted(async () => {
  await updatePermissions();
});
</script>

<style scoped>
.disclosure {
  @apply relative text-xs text-naturals-N11 bg-naturals-N1 font-bold py-3 px-4 cursor-pointer hover:text-naturals-N14 transition-colors duration-200 select-none;
}
</style>
