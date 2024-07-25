<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col gap-2 h-1/2">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Update Talos on Node {{ $route.query.machine }}</h3>
      <close-button @click="close" />
    </div>
    <managed-by-templates-warning warning-style="popup"/>
    <template v-if="!loading && machine">
      <radio-group id="k8s-upgrade-version" class="text-naturals-N13 overflow-y-auto flex-1 flex flex-col gap-2" v-model="selectedVersion">
        <template v-for="(versions, group) in upgradeVersions" :key="group">
          <radio-group-label as="div" class="pl-7 font-bold bg-naturals-N4 w-full p-1 text-sm">{{ group }}</radio-group-label>
          <div class="flex flex-col gap-1">
            <radio-group-option v-for="version in versions" :key="version" v-slot="{ checked }" :value="version">
              <div class="flex items-center gap-2 cursor-pointer text-sm px-2 py-1 hover:bg-naturals-N4 tranform transition-color" :class="{ 'bg-naturals-N4': checked }">
                <t-checkbox :checked="checked"/>
                {{ version }}
                <span v-if="version === machine.spec.talos_version?.slice(1)">(current)</span>
              </div>
            </radio-group-option>
          </div>
        </template>
      </radio-group>
    </template>
    <div v-else class="flex-1"/>

    <div class="flex justify-end gap-4">
      <t-button @click="upgradeClick" class="w-32 h-9" :disabled="!versions || updating" type="highlighted">
        <t-spinner v-if="!versions || updating" class="w-5 h-5" />
        <span v-else>Update</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, computed, watch } from "vue";
import { DefaultNamespace, MachineStatusType, TalosVersionType } from "@/api/resources";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import * as semver from "semver";
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import { MachineStatusSpec, TalosVersionSpec } from "@/api/omni/specs/omni.pb";
import { Resource } from "@/api/grpc";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";
import Watch from "@/api/watch";
import { updateTalosMaintenance } from "@/methods/machine";
import { showError, showSuccess } from "@/notification";

const router = useRouter();
const route = useRoute();

const selectedVersion = ref("");

const versions: Ref<Resource<TalosVersionSpec>[]> = ref([]);

const versionsWatch = new Watch(versions);

versionsWatch.setup({
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni
});

const machine = ref<Resource<MachineStatusSpec>>();

const machineWatch = new Watch(machine);

machineWatch.setup({
  resource: {
    type: MachineStatusType,
    namespace: DefaultNamespace,
    id: route.query.machine as string,
  },
  runtime: Runtime.Omni
});

watch(machine, () => {
  selectedVersion.value = machine.value?.spec.talos_version?.slice(1) ?? "";
})

const loading = versionsWatch.loading;
const updating = ref(false);

const upgradeVersions = computed(() => {
  const sorted = new Array<Resource<TalosVersionSpec>>(...versions.value);

  sorted.sort((a, b) => {
    return semver.compare(a.metadata.id, b.metadata.id);
  });

  const result: Record<string, string[]> = {};

  for (const version of sorted) {
    if (version.spec.deprecated) {
      continue;
    }

    const major = semver.major(version.metadata.id);
    const minor = semver.minor(version.metadata.id);

    const majorMinor = `${major}.${minor}`;

    if (!result[majorMinor]) {
      result[majorMinor] = [];
    }

    result[majorMinor].push(version.metadata.id!);
  }

  return result;
});


let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const upgradeClick = async () =>  {
  if (machine.value?.spec.talos_version === `v${selectedVersion.value}`) {
    return;
  }

  updating.value = true;

  try {
    await updateTalosMaintenance(route.query.machine as string, selectedVersion.value, machine.value?.spec.schematic?.id);
  } catch (e) {
    showError("Failed to Do Maintenance Update", e.message)

    return;
  } finally {
    updating.value = false;

    close();
  }

  showSuccess("The Machine Update Triggered", `Machine ${route.query.machine} is being updated to Talos version ${selectedVersion.value}`)
};
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
optgroup {
  @apply text-naturals-N14 font-bold;
}
</style>
