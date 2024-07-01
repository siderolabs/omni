<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="h-screen py-4 gap-2 z-30 overflow-hidden flex items-center justify-center" :class="{ 'w-3/5': Object.keys(sourceToProgress).length > 0 }">
    <div class="rounded bg-naturals-N2 p-8 flex flex-col min-w-fit w-full max-h-full gap-2">
      <div class="heading">
        <h3 class="text-base text-naturals-N14">Download Support Bundle</h3>
        <close-button @click="close"/>
      </div>

      <div class="flex-1 overflow-y-auto pr-2">
        <template v-if="sortedSources.length > 0">
          <div v-for="(source) in sortedSources" :key="source" class="rounded-md text-xs my-1 border border-naturals-N6 flex flex-col divide-naturals-N6 divide-y overflow-hidden">
            <div class="flex gap-2 px-3 bg-naturals-N3 p-3">
              <icon-button icon="arrow-up" class="transition-transform" :class="{'rotate-180': expanded[source]}" @click="() => expanded[source] = !expanded[source]"/>
              <div class="list-grid text-naturals-N12 flex-1">
                <div class="truncate">{{ source }}</div>
                <div class="flex gap-2 col-span-2 items-center">
                  <t-icon class="w-4 h-4" :icon="sourceToProgress[source].icon" :style="{ 'color': sourceToProgress[source].color }"/>
                  <progress-bar class="flex-1" :progress="sourceToProgress[source].progress" :color="sourceToProgress[source].color"/>
                </div>
              </div>
            </div>
            <div v-if="expanded[source]" class="py-4">
              <tooltip v-for="state in sourceToProgress[source].states" :key="state.text" :description="state.error">
                <div class="flex gap-2 items-center px-4 py-0.5 cursor-pointer hover:bg-naturals-N4">
                  <t-icon
                    class="w-4 h-4"
                    :class="{'text-green-G1': state.error === undefined, 'text-red-R1': state.error}"
                    :icon="state.error ? 'warning' : 'check'"/>
                  <div>{{ state.text }}</div>
                </div>
              </tooltip>
            </div>
          </div>
        </template>
      </div>

      <div class="flex justify-end gap-4">
        <t-button @click="download" class="w-32 h-9" :disabled="!canDownloadSupportBundle || started" type="highlighted">
          <t-spinner v-if="!status" class="w-5 h-5"/>
          <span v-else>Download</span>
        </t-button>
        <t-button @click="close" class="w-32 h-9">
          <t-spinner v-if="!status" class="w-5 h-5"/>
          <span v-else>{{ closeText }}</span>
        </t-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, computed, watch, onUnmounted } from "vue";
import { DefaultNamespace, TalosUpgradeStatusType } from "@/api/resources";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";
import { TalosUpgradeStatusSpec } from "@/api/omni/specs/omni.pb";
import { Resource } from "@/api/grpc";
import { setupClusterPermissions } from "@/methods/auth";
import { ManagementService } from "@/api/omni/management/management.pb";
import { showError } from "@/notification";
import { b64Decode } from "@/api/fetch.pb";
import { green, red, yellow } from "@/vars/colors";
import { withAbortController } from "@/api/options";

import ProgressBar from "@/components/common/ProgressBar/ProgressBar.vue";
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import Watch from "@/api/watch";
import IconButton from "@/components/common/Button/IconButton.vue";
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";

const route = useRoute();
const router = useRouter();
const expanded = ref<Record<string, boolean>>({});

const selectedVersion = ref("");

const clusterName = route.params.cluster as string;

const resource = {
  namespace: DefaultNamespace,
  type: TalosUpgradeStatusType,
  id: clusterName,
};

const status: Ref<Resource<TalosUpgradeStatusSpec> | undefined> = ref();

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

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

interface State {
  error?: string
  text?: string
}

interface Progress {
  states: State[]
  currentNum: number
  progress: number
  icon: IconType
  color: string
}

const sourceToProgress: Ref<Record<string, Progress>> = ref({});
const sortedSources = computed(() => Object.keys(sourceToProgress.value).sort());

const started = ref(false);
const done = ref(false);

const closeText = computed(() => {
  return done.value ? "Close" : "Cancel";
});

const abortController = new AbortController()

let canceled = false

onUnmounted(() => {
  abortController.abort();

  canceled = true
});

const download = async () => {
  try {
    started.value = true;
    sourceToProgress.value = {};

    await ManagementService.GetSupportBundle({
      cluster: clusterName,
    }, resp => {
      if (resp?.progress?.source) {
        let current = sourceToProgress.value[resp.progress.source];

        if (!current) {
          current = {
            states: [],
            currentNum: 0,
            progress: 0,
            icon: "loading",
            color: yellow.Y1
          }

          sourceToProgress.value[resp.progress.source] = current;
        }

        current.currentNum += 1;
        current.progress = current.currentNum / (resp.progress.total ?? 1) * 100

        if (resp.progress.state) {
          current.states.push({
            error: resp.progress.error,
            text: resp.progress.state
          })
        }

        if (resp.progress.error) {
          current.color = red.R1
          current.icon = "warning"
        } else if(current.progress === 100 && current.color !== red.R1) {
          current.color = green.G1
          current.icon = "check-in-circle-classic"
        }
      }

      if (resp.bundle_data) {
        const data = resp.bundle_data as unknown as string; // bundle_data is actually not a Uint8Array, but a base64 string
        const rawData = b64Decode(data);
        const blob = new Blob([rawData], { type: "application/zip" });

        const url = window.URL.createObjectURL(blob);
        downloadURL(url, "support.zip");

        done.value = true;

        return;
      }
    }, withAbortController(abortController));
  } catch (e) {
    if (canceled) {
      return;
    }

    showError("Download Failed", e.message);
  }
}

const downloadURL = (data: string, fileName: string) => {
  const a = document.createElement('a')
  a.href = data
  a.download = fileName
  document.body.appendChild(a)
  a.style.display = 'none'
  a.click()
  a.remove()
}

const { canDownloadSupportBundle } = setupClusterPermissions(computed(() => route.params.cluster as string))
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

optgroup {
  @apply text-naturals-N14 font-bold;
}

.list-grid {
  @apply grid grid-cols-3 items-center justify-center gap-2;
}

.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs;
}
</style>
