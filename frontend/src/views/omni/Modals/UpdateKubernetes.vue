<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col max-h-screen my-4 gap-2">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Update Kubernetes</h3>
      <close-button @click="close" />
    </div>
    <managed-by-templates-warning warning-style="popup"/>
    <template v-if="status">
      <radio-group id="k8s-upgrade-version" class="text-naturals-N13 overflow-y-auto flex-1 flex flex-col gap-2" v-model="selectedVersion">
        <template v-for="(versions, group) in upgradeVersions" :key="group">
          <radio-group-label as="div" class="pl-7 font-bold bg-naturals-N4 w-full p-1 text-sm">{{ group }}</radio-group-label>
          <div class="flex flex-col gap-1">
            <radio-group-option v-for="version in versions" :key="version" v-slot="{ checked }" :value="version">
              <div class="flex items-center gap-2 cursor-pointer text-sm px-2 py-1 hover:bg-naturals-N4 tranform transition-color" :class="{ 'bg-naturals-N4': checked }">
                <t-checkbox :checked="checked"/>
                {{ version }}
                <span v-if="version === status.spec.last_upgrade_version">(current)</span>
              </div>
            </radio-group-option>
          </div>
        </template>
      </radio-group>
    </template>

    <p class="text-xs">
      Changing the Kubernetes version can result in control plane downtime. During this change you will be able to cancel the upgrade.
    </p>
    <p class="text-xs">
      This operation starts immediately.
    </p>

    <div v-if="runningPrechecks" class="text-primary-P3 text-xs">
       Running pre-checks to validate the upgrade...
    </div>

    <div v-if="preCheckError" class="text-primary-P3 text-xs font-mono whitespace-pre-line">
       {{ preCheckError }}
    </div>

    <div class="flex justify-end gap-4">
      <t-button @click="upgradeClick" class="w-32 h-9" :disabled="!status || runningPrechecks || selectedVersion === status?.spec?.last_upgrade_version" type="highlighted">
        <t-spinner v-if="runningPrechecks || !status" class="w-5 h-5" />
        <span v-else>{{ action }}</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, computed, watch } from "vue";
import { DefaultNamespace, KubernetesUpgradeStatusType } from "@/api/resources";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { Resource } from "@/api/grpc";
import { upgradeKubernetes } from "@/methods/cluster";
import * as semver from "semver";
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import Watch from "@/api/watch";
import { KubernetesUpgradeStatusSpec } from "@/api/omni/specs/omni.pb";
import { ManagementService } from "@/api/omni/management/management.pb";
import { withContext } from "@/api/options";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";

const route = useRoute();
const router = useRouter();

const runningPrechecks = ref(false);
const preCheckError = ref("");
const selectedVersion = ref("");

const clusterName = route.params.cluster as string;

const resource = {
  namespace: DefaultNamespace,
  type: KubernetesUpgradeStatusType,
  id: clusterName,
};

const status: Ref<Resource<KubernetesUpgradeStatusSpec> | undefined> = ref();

const upgradeStatusWatch = new Watch(status);

upgradeStatusWatch.setup({
  resource: resource,
  runtime: Runtime.Omni,
});

watch(status, () => {
  if (selectedVersion.value === "") {
    selectedVersion.value = status.value?.spec.last_upgrade_version || "";
  }
});

const upgradeVersions = computed(() => {
  if (!status?.value?.spec?.upgrade_versions) {
    return [];
  }

  const sorted = [...status.value.spec.upgrade_versions, status.value.spec.last_upgrade_version].sort(semver.compare);

  const result = {};

  for (const version of sorted) {
    const major = semver.major(version);
    const minor = semver.minor(version);

    const majorMinor = `${major}.${minor}`;

    if (!result[majorMinor]) {
      result[majorMinor] = [];
    }

    result[majorMinor].push(version);
  }

  return result;
});

const action = computed(() => {
  if (!status.value) {
    return "Loading...";
  }

  switch (semver.compare(selectedVersion.value, status.value.spec.last_upgrade_version)) {
    case 1:
      return "Upgrade";
    case -1:
      return "Downgrade";
  }

  return "Unchanged";
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
  runningPrechecks.value = true;
  preCheckError.value = "";

  try {
    const response = await ManagementService.KubernetesUpgradePreChecks(
      {
        new_version: selectedVersion.value,
      }, withContext({cluster: clusterName})
    );

    if (response.ok) {
      upgradeKubernetes(clusterName, selectedVersion.value);

      close();
    } else {
      preCheckError.value = response.reason!;
    }
  } catch (e) {
    preCheckError.value = e.message || e.toString();
  }

  runningPrechecks.value = false
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
