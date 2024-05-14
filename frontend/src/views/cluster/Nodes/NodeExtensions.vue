<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col flex-1">
    <div class="flex flex-col gap-4 flex-1 pb-10">
      <t-input v-model="searchString" icon="search"/>
      <div class="flex-1 flex flex-col">
        <template v-if="ready && extensionsState.length > 0">
          <div class="header list-grid">
            <div>Name</div>
            <div>State</div>
            <div>Level</div>
          </div>
          <t-list-item v-for="item in extensionsState" :key="item.name">
            <div class="flex gap-2 px-3">
              <div class="list-grid text-naturals-N12 flex-1">

              <WordHighlighter
                  :query="searchString"
                  :textToHighlight="item.name"
                  highlightClass="bg-naturals-N14"
                  class="text-naturals-N14"/>
                <div class="flex">
                  <div class="flex gap-2 bg-naturals-N3 text-naturals-N13 rounded px-2 py-1 items-center text-xs">
                    <template v-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installing">
                      <t-icon icon="loading" class="text-yellow-Y1 w-4 h-4 animate-spin"/>
                      <span>Installing</span>
                    </template>
                    <template v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Removing">
                      <t-icon icon="delete" class="text-red-R1 w-4 h-4 animate-pulse"/>
                      <span>Removing</span>
                    </template>

                    <template v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installed">
                      <t-icon icon="check-in-circle" class="text-green-G1 w-4 h-4"/>
                      <span >Installed</span>
                    </template>
                  </div>
                </div>
                <div v-if="item.source">
                  {{ item.source }}
                </div>
              </div>
            </div>
          </t-list-item>
        </template>
        <div v-else-if="!ready" class="flex-1 flex items-center justify-center">
          <t-spinner class="w-6 h-6"/>
        </div>
        <div v-else class="flex-1">
          <t-alert title="No Extensions Found" type="info"/>
        </div>
      </div>
    </div>
    <div class="sticky -bottom-6 -my-6 -mx-6 bg-naturals-N1 border-t border-naturals-N5 h-16 flex items-center justify-end gap-2 px-12 py-6 text-xs">
      <t-button type="highlighted"
        @click="openExtensionsUpdate"
        >
        Update Extensions
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { Resource } from "@/api/grpc";
import { DefaultNamespace, ExtensionsConfigurationType, MachineExtensionsType, ExtensionsConfigurationLabel, LabelCluster, LabelMachineSet, LabelClusterMachine, MachineExtensionsStatusType } from "@/api/resources";
import Watch, { WatchOptions } from "@/api/watch";
import TListItem from "@/components/common/List/TListItem.vue";
import { computed, ref } from "vue";
import { ExtensionsConfigurationSpec, MachineExtensionsSpec, MachineExtensionsStatusSpec, MachineExtensionsStatusSpecItemPhase } from "@/api/omni/specs/omni.pb";
import { useRoute, useRouter } from "vue-router";

import TInput from "@/components/common/TInput/TInput.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TAlert from "@/components/TAlert.vue";
import WordHighlighter from "vue-word-highlighter";

const machineExtensionsStatus = ref<Resource<MachineExtensionsStatusSpec>>();
const machineExtensionsStatusWatch = new Watch(machineExtensionsStatus);
const searchString = ref("");
const router = useRouter();
const route = useRoute();

const machineExtensionsStatusWatchLoading = machineExtensionsStatusWatch.loading;

const ready = computed(() => {
  return !machineExtensionsStatusWatchLoading.value;
});

machineExtensionsStatusWatch.setup(computed((): WatchOptions | undefined => {
  return {
    resource: {
      namespace: DefaultNamespace,
      type: MachineExtensionsStatusType,
      id: route.params.machine as string,
    },
    runtime: Runtime.Omni
  }
}));

const machineExtensions = ref<Resource<MachineExtensionsSpec>>()

const machineExtensionsWatch = new Watch(machineExtensions);

machineExtensionsWatch.setup(computed(() => {
  if (!machineExtensionsStatus.value) {
    return;
  }

  return {
    resource: {
      id: machineExtensionsStatus.value.metadata.id!,
      namespace: DefaultNamespace,
      type: MachineExtensionsType,
    },
    runtime: Runtime.Omni
  };
}));

const extensionsConfiguration = ref<Resource<ExtensionsConfigurationSpec>>();

const configurationsWatch = new Watch(extensionsConfiguration);

configurationsWatch.setup(computed((): WatchOptions | undefined => {
  const id = machineExtensions.value?.metadata.labels?.[ExtensionsConfigurationLabel];
  if (!id) {
    return;
  }

  return {
    resource: {
      namespace: DefaultNamespace,
      type: ExtensionsConfigurationType,
      id: id,
    },
    runtime: Runtime.Omni,
  };
}));

const extensionsState = computed(() => {
  if (!machineExtensionsStatus.value?.spec.extensions) {
    return [];
  }

  type Item = {
    name: string
    source: string | null,
    phase: MachineExtensionsStatusSpecItemPhase
  }

  const res: Item[] = [];

  for (const extension of machineExtensionsStatus.value.spec.extensions) {
    if (searchString.value !== "" && !extension.name?.includes(searchString.value)) {
      continue;
    }

    res.push({
      name: extension.name!,
      source: extension.immutable ? "Automatically installed by Omni" : extensionsLevel.value,
      phase: extension.phase ?? MachineExtensionsStatusSpecItemPhase.Installed,
    });
  }

  return res;
});

const extensionsLevel = computed(() => {
  if (!extensionsConfiguration.value) {
    return null;
  }

  for (const label of [LabelClusterMachine, LabelMachineSet, LabelCluster]) {
    const value = extensionsConfiguration.value.metadata.labels?.[label];
    if (value) {
      switch (label) {
        case LabelClusterMachine:
          return `Defined for the machine ${value}`;
        case LabelMachineSet:
          return `Inherited from the machine set ${value}`;
        case LabelCluster:
          return `Inherited from the cluster ${value}`;
      }
    }
  }

  return null;
});

const openExtensionsUpdate = () => {
  router.push({ query: {
    machine: route.params.machine as string,
    modal: "updateExtensions"
  }});
}
</script>

<style scoped>
.list-grid {
  @apply grid grid-cols-3 items-center justify-center;
}

.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs;
}
</style>
